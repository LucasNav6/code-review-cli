package claudereview

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
)

// Colours holds the raw hex values that drive the renderer. Pulling
// them in directly (instead of using lipgloss.Style values) keeps the
// renderer compatible with lipgloss v2 where Style is opaque.
type Colours struct {
	Foreground string // white-ish, used for borders and primary text
	Muted      string // grey, used for context lines and bullet markers
	Added      string // green, used for "+" lines
	Removed    string // red, used for "-" lines
	Category   string // accent colour for the finding title
	Accent     string // colour for section labels like "Impact:", "Suggestion:"
}

// Renderer formats a parsed Finding as a comment-style block that
// pairs the source snippets claude picked with the narrative it
// produced. The current implementation is deliberately minimal
// (plain text with light lipgloss colouring) — a future iteration
// will wrap each finding in a left-border box.
type Renderer struct {
	c Colours
}

// NewRenderer builds a renderer from the project's colour palette.
// The expected values are the hex strings already used in ui/theme.go
// (e.g. "#FAFAFA", "#8B949E", "#3FB950", "#F85149"). No new colours
// are introduced.
func NewRenderer(c Colours) *Renderer {
	return &Renderer{c: c}
}

// Render writes a single Finding as a self-contained block. The
// snippet list comes straight from claude (parsed from JSON) so there
// is no longer any need to look up a hunk by file + line: the renderer
// is deterministic given the JSON payload.
func (r *Renderer) Render(w io.Writer, f Finding) {
	r.renderHeader(w, f)
	r.renderSnippets(w, f)
	r.renderContext(w, f)
	r.renderImpact(w, f)
	r.renderSuggestion(w, f)
}

// renderHeader writes the title line: `CATEGORY  title  file:line`.
func (r *Renderer) renderHeader(w io.Writer, f Finding) {
	categoryStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(r.c.Category))
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(r.c.Foreground))
	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(r.c.Muted))

	category := strings.ToUpper(f.Category)
	if category == "" {
		category = "FINDING"
	}

	lineRef := ""
	if f.File != "" {
		if f.Line > 0 {
			lineRef = fmt.Sprintf("  %s:%d", f.File, f.Line)
		} else {
			lineRef = "  " + f.File
		}
	}

	fmt.Fprint(w, categoryStyle.Render(category))
	fmt.Fprint(w, "  ")
	fmt.Fprint(w, titleStyle.Render(f.Title))
	fmt.Fprintln(w, pathStyle.Render(lineRef))
}

// renderSnippets writes the diff snippets claude returned, each as a
// coloured one-line block. Prefixes (+, -, space) drive the colour.
func (r *Renderer) renderSnippets(w io.Writer, f Finding) {
	if len(f.Snippets) == 0 {
		return
	}

	added := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Added))
	removed := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Removed))
	context := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Muted))

	gutterWidth := 0
	for _, s := range f.Snippets {
		if s.Line > 0 && len(fmt.Sprintf("%d", s.Line)) > gutterWidth {
			gutterWidth = len(fmt.Sprintf("%d", s.Line))
		}
	}

	fmt.Fprintln(w)
	for _, s := range f.Snippets {
		code := s.Code
		var style lipgloss.Style
		switch {
		case strings.HasPrefix(code, "+"):
			style = added
		case strings.HasPrefix(code, "-"):
			style = removed
		default:
			style = context
		}

		var gutter string
		if s.Line > 0 {
			gutter = fmt.Sprintf("%*d ", gutterWidth, s.Line)
		} else {
			gutter = strings.Repeat(" ", gutterWidth+1)
		}

		fmt.Fprint(w, lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Muted)).Render(gutter))
		fmt.Fprintln(w, style.Render(code))
	}
}

// renderContext writes the short problem statement as one paragraph.
func (r *Renderer) renderContext(w io.Writer, f Finding) {
	if f.Context == "" {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, f.Context)
}

// renderImpact writes the bullet list of consequences. Empty items
// are skipped so a noisy prompt does not produce visual clutter.
func (r *Renderer) renderImpact(w io.Writer, f Finding) {
	if len(f.Impact) == 0 {
		return
	}

	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(r.c.Accent))
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Foreground))

	fmt.Fprintln(w)
	fmt.Fprintln(w, accentStyle.Render("Impact"))
	for _, item := range f.Impact {
		if strings.TrimSpace(item) == "" {
			continue
		}
		fmt.Fprint(w, lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Muted)).Render("  • "))
		fmt.Fprintln(w, bodyStyle.Render(item))
	}
}

// renderSuggestion writes the suggested fix as the final block.
func (r *Renderer) renderSuggestion(w io.Writer, f Finding) {
	if f.Suggestion == "" {
		return
	}

	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(r.c.Accent))
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Foreground))

	fmt.Fprintln(w)
	fmt.Fprintln(w, accentStyle.Render("Suggestion"))
	fmt.Fprintln(w, bodyStyle.Render(f.Suggestion))
}

// RenderNoFindings is the friendly message emitted when claude
// returned an empty findings list.
func (r *Renderer) RenderNoFindings(w io.Writer) {
	check := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(r.c.Added))
	body := check.Render("✓ ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Muted)).Render("No resilience findings")
	fmt.Fprintln(w, body)
}
