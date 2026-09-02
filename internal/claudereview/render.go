package claudereview

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/LucasNav6/code-review-cli/ui"
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

// Renderer formats a parsed Finding as a styled comment-style card
// inspired by GitHub review comments: rounded border, a coloured
// title line, the source snippets with line numbers and diff
// prefixes, then the structured impact/suggestion blocks.
//
// The same renderer drives all four review categories (RESILIENCE,
// READABILITY, SECURITY, TESTING). Category-specific fields
// (OWASP for SECURITY; RequiresTests/TestsCovered/TestsMissing/EdgeCase
// for TESTING) are rendered in addition to the common blocks when
// they are present in the Finding.
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

// Render writes a single Finding as a self-contained card. lipgloss
// owns the border and word-wraps the body to fit the requested
// width, so we just feed it pre-coloured lines.
func (r *Renderer) Render(w io.Writer, f Finding) {
	rendered := r.renderBox(f)
	fmt.Fprintln(w, rendered)
}

// renderBox composes the inner body and wraps it in a rounded box
// sized to the terminal width. lipgloss handles all word wrapping
// and border alignment — we just hand it pre-styled strings.
func (r *Renderer) renderBox(f Finding) string {
	body := r.renderBody(f)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(r.c.Foreground)).
		Width(ui.TerminalWidth()).
		Padding(0, 1)

	return box.Render(body)
}

// renderBody concatenates the title, snippets, context, impact,
// category-specific blocks and suggestion. Section separators are
// single blank lines so the card reads as a series of paragraphs.
// lipgloss wraps each line to the available width, so we never
// pre-wrap text here.
func (r *Renderer) renderBody(f Finding) string {
	var b strings.Builder

	b.WriteString(r.renderHeaderBlock(f))

	if len(f.Snippets) > 0 {
		b.WriteByte('\n')
		for _, line := range r.renderSnippetLines(f) {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	if strings.TrimSpace(f.Context) != "" {
		b.WriteByte('\n')
		b.WriteString(muted(r.c.Muted, f.Context))
		b.WriteByte('\n')
	}

	if len(f.Impact) > 0 {
		b.WriteByte('\n')
		b.WriteString(bold(r.c.Foreground, "Impact"))
		b.WriteByte('\n')
		for _, item := range f.Impact {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			b.WriteString(muted(r.c.Muted, "  • "+item))
			b.WriteByte('\n')
		}
	}

	// Category-specific blocks: SECURITY shows the OWASP label,
	// TESTING shows the four testing-helper lines.
	b.WriteString(r.renderCategoryExtras(f))

	if strings.TrimSpace(f.Suggestion) != "" {
		b.WriteByte('\n')
		b.WriteString(bold(r.c.Foreground, "Suggestion"))
		b.WriteByte('\n')
		b.WriteString(muted(r.c.Muted, f.Suggestion))
		b.WriteByte('\n')
	}

	return b.String()
}

// renderCategoryExtras emits the per-category auxiliary blocks. Each
// block starts with its own blank-line separator so the visual rhythm
// of the card stays the same as the common blocks above.
//
// Returns "" for categories that have no extras (RESILIENCE,
// READABILITY) so the caller can append unconditionally.
func (r *Renderer) renderCategoryExtras(f Finding) string {
	var b strings.Builder

	switch f.Category {
	case CategorySecurity:
		if f.OWASP != "" {
			b.WriteByte('\n')
			b.WriteString(bold(r.c.Foreground, "OWASP"))
			b.WriteByte('\n')
			b.WriteString(muted(r.c.Muted, "  "+f.OWASP))
			b.WriteByte('\n')
		}
	case CategoryTesting:
		b.WriteString(r.renderTestingExtras(f))
	}

	return b.String()
}

// renderTestingExtras builds the four auxiliary lines a TESTING
// finding carries: requires_tests, tests_covered, tests_missing,
// edge_case. We render them as a "labelled list" so the user can scan
// them quickly without parsing prose.
func (r *Renderer) renderTestingExtras(f Finding) string {
	var b strings.Builder

	// requires_tests is the only mandatory-looking block: a TESTING
	// finding without it is almost certainly a model mistake.
	if f.RequiresTests == nil {
		return ""
	}

	b.WriteByte('\n')
	b.WriteString(bold(r.c.Foreground, "Testing"))
	b.WriteByte('\n')

	if *f.RequiresTests {
		b.WriteString(muted(r.c.Muted, "  Requires tests: yes"))
	} else {
		b.WriteString(muted(r.c.Muted, "  Requires tests: no"))
	}
	b.WriteByte('\n')

	if f.TestsCovered != "" {
		b.WriteString(muted(r.c.Muted, "  Tests covered: "+f.TestsCovered))
		b.WriteByte('\n')
	}
	if f.TestsMissing != "" {
		b.WriteString(muted(r.c.Muted, "  Tests missing: "+f.TestsMissing))
		b.WriteByte('\n')
	}
	if f.EdgeCase != "" {
		b.WriteString(muted(r.c.Muted, "  Edge case: "+f.EdgeCase))
		b.WriteByte('\n')
	}

	return b.String()
}

// renderHeaderBlock returns the three coloured lines that open the
// card. Each line is rendered separately so lipgloss cannot merge
// them when the path is long:
//
//	CATEGORY
//	Title
//	file:line
//
// The newline between them is intentional — the caller appends
// subsequent blocks with their own blank-line separator.
func (r *Renderer) renderHeaderBlock(f Finding) string {
	category := strings.ToUpper(f.Category)
	if category == "" {
		category = "FINDING"
	}

	head := bold(r.c.Category, category) + "\n" +
		bold(r.c.Foreground, f.Title)

	if f.File != "" {
		if f.Line > 0 {
			head += "\n" + muted(r.c.Muted, fmt.Sprintf("%s:%d", f.File, f.Line))
		} else {
			head += "\n" + muted(r.c.Muted, f.File)
		}
	}
	return head
}

// renderSnippetLines formats the snippets so each snippet becomes a
// block of "<line-no> <prefixed-code>" rows. Prefixes (+, -, space)
// drive the colour. When claude omits the diff prefix we assume the
// line was added (most common case for resilience findings) and
// colour it green so the user always sees a meaningful diff.
func (r *Renderer) renderSnippetLines(f Finding) []string {
	if len(f.Snippets) == 0 {
		return nil
	}

	gutterWidth := 0
	for _, s := range f.Snippets {
		if s.Line > 0 {
			if n := len(fmt.Sprintf("%d", s.Line)); n > gutterWidth {
				gutterWidth = n
			}
		}
	}

	added := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Added))
	removed := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Removed))
	neutral := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Foreground))

	out := make([]string, 0, len(f.Snippets))
	for _, s := range f.Snippets {
		var gutter string
		if s.Line > 0 {
			gutter = muted(r.c.Muted, fmt.Sprintf("%*d ", gutterWidth, s.Line))
		} else {
			gutter = strings.Repeat(" ", gutterWidth+1)
		}

		var code lipgloss.Style
		var display string
		switch {
		case strings.HasPrefix(s.Code, "-"):
			code = removed
			display = s.Code
		case strings.HasPrefix(s.Code, "+"):
			code = added
			display = s.Code
		case strings.HasPrefix(s.Code, " "):
			code = neutral
			display = s.Code
		default:
			// No prefix supplied by claude — treat as an added line
			// and prepend the "+" so the diff colour is meaningful.
			code = added
			display = "+ " + s.Code
		}

		out = append(out, gutter+code.Render(display))
	}
	return out
}

// RenderNoFindings is the friendly message emitted when claude
// returned an empty findings list. It is rendered with the same
// rounded-border look as a finding so the visual rhythm is
// consistent.
func (r *Renderer) RenderNoFindings(w io.Writer) {
	check := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(r.c.Added))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Muted))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(r.c.Foreground)).
		Width(ui.TerminalWidth()).
		Padding(0, 1)

	body := check.Render("✓ ") + text.Render("No review findings")
	fmt.Fprintln(w, box.Render(body))
}

// bold returns the given text rendered in bold with the supplied
// foreground colour.
func bold(colour, s string) string {
	if s == "" {
		return ""
	}
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colour)).Render(s)
}

// fg returns the given text rendered with the supplied foreground
// colour.
func fg(colour, s string) string {
	if s == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colour)).Render(s)
}

// muted returns the given text rendered in the muted colour.
func muted(colour, s string) string {
	if s == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colour)).Render(s)
}
