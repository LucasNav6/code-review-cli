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
	Muted      string // grey, used for context lines
	Added      string // green, used for "+" lines
	Removed    string // red, used for "-" lines
	Category   string // accent colour for the finding title
}

// Renderer formats a parsed Finding as a comment-style block that
// pairs the relevant diff hunk with claude's narrative. Output is
// plain text with ANSI colour codes; non-TTY consumers can strip
// them.
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

// Render writes a single Finding as a self-contained block. When no
// hunk is supplied (claude referenced a file that does not appear in
// the diff), the comment is rendered without the code preview so the
// user still gets the narrative.
func (r *Renderer) Render(w io.Writer, f Finding, hunk *Hunk) {
	fmt.Fprintln(w, r.renderHeader(f))
	fmt.Fprintln(w)

	if hunk != nil {
		fmt.Fprintln(w, r.renderHunk(*hunk))
		fmt.Fprintln(w)
	}

	r.renderFields(w, f)
}

// renderHeader is the title line: `<category>  archivo:línea`.
func (r *Renderer) renderHeader(f Finding) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(r.c.Category))
	pathStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(r.c.Foreground))

	category := strings.ToUpper(f.Categoria)
	if category == "" {
		category = "FINDING"
	}
	lineRef := fmt.Sprintf("%s:%d", f.Archivo, f.Linea)

	return titleStyle.Render(category) + "  " + pathStyle.Render(lineRef)
}

// renderHunk formats the diff body with the standard +/- colouring.
// Each line keeps its original prefix so the reader can see at a
// glance which lines were added, removed or kept as context.
func (r *Renderer) renderHunk(h Hunk) string {
	added := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Added))
	removed := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Removed))
	context := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Muted))

	var b strings.Builder
	for _, line := range h.Lines {
		var style lipgloss.Style
		switch {
		case strings.HasPrefix(line, "+"):
			style = added
		case strings.HasPrefix(line, "-"):
			style = removed
		default:
			style = context
		}

		b.WriteString(style.Render(line))
		b.WriteByte('\n')
	}

	return strings.TrimRight(b.String(), "\n")
}

// renderFields prints the narrative paragraphs one after the other,
// indented so they read as a coherent comment block.
func (r *Renderer) renderFields(w io.Writer, f Finding) {
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Foreground))
	indent := "  "
	write := func(label, body string) {
		if body == "" {
			return
		}
		fmt.Fprint(w, text.Render(indent+label+": "))
		fmt.Fprintln(w, text.Render(body))
		fmt.Fprintln(w)
	}

	write("Comportamiento ante fallo", f.ComportamientoFallo)
	write("Observabilidad", f.Observabilidad)
	write("Comentario", f.Comentario)
}

// RenderNoFindings is the friendly message emitted when claude
// returned "NO_FINDINGS" or an empty response.
func (r *Renderer) RenderNoFindings(w io.Writer) {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(r.c.Foreground)).
		Padding(0, 1)

	check := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(r.c.Added))
	body := check.Render("✓ ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color(r.c.Muted)).Render("No resilience findings")
	fmt.Fprintln(w, box.Render(body))
}
