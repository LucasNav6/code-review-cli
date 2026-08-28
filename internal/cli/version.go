package cli

import (
	"fmt"
	"runtime"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/LucasNav6/code-review-cli/internal/buildinfo"
)

// versionText arma la card que muestra `code-review --version`.
func versionText() string {
	header := brandStyle.Render("code-review") + "  " + mutedStyle.Render(buildinfo.Version)

	rows := [][2]string{
		{"Commit", buildinfo.Commit},
		{"Built", buildinfo.Date},
		{"Go", strings.TrimPrefix(runtime.Version(), "go")},
		{"Platform", runtime.GOOS + "/" + runtime.GOARCH},
	}

	return renderCard(header, rows) + "\n"
}

// renderCard dibuja una card con un encabezado y una tabla de filas
// etiqueta/valor, separadas por un divisor — sin depender de colores, solo
// de bordes y espaciado.
func renderCard(header string, rows [][2]string) string {
	labelWidth := 0

	for _, row := range rows {
		labelWidth = max(labelWidth, len(row[0]))
	}

	lines := make([]string, 0, len(rows))

	for _, row := range rows {
		label := mutedStyle.Render(fmt.Sprintf("%-*s", labelWidth, row[0]))
		lines = append(lines, label+"   "+row[1])
	}

	contentWidth := lipgloss.Width(header)

	for _, line := range lines {
		contentWidth = max(contentWidth, lipgloss.Width(line))
	}

	borderStyle := lipgloss.NewStyle().Foreground(border)

	line := func(left, fill, right string) string {
		return borderStyle.Render(left + strings.Repeat(fill, contentWidth+2) + right)
	}

	side := borderStyle.Render("│")

	pad := func(content string) string {
		gap := contentWidth - lipgloss.Width(content)
		return side + " " + content + strings.Repeat(" ", gap) + " " + side
	}

	var b strings.Builder

	b.WriteString(line("╭", "─", "╮") + "\n")
	b.WriteString(pad(header) + "\n")
	b.WriteString(line("├", "─", "┤") + "\n")

	for _, l := range lines {
		b.WriteString(pad(l) + "\n")
	}

	b.WriteString(line("╰", "─", "╯"))

	return b.String()
}
