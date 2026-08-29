package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/LucasNav6/code-review-cli/internal/buildinfo"
	"github.com/LucasNav6/code-review-cli/internal/review"
)

func (m Model) View() tea.View {
	content := m.render()

	v := tea.NewView(content)
	v.AltScreen = true

	return v
}

func (m Model) render() string {
	if m.width == 0 {
		return "\n  Preparando interfaz..."
	}

	totalWidth := max(70, m.width-2)

	if m.preflight != preflightPassed {
		return lipgloss.NewStyle().
			Padding(1, 2).
			Render(m.renderPreflightScreen(totalWidth))
	}

	header := m.renderHeader(totalWidth)

	// Un error antes de que arranque el pipeline (falló traer el PR o el
	// diff) todavía no tiene tabs ni secciones que mostrar.
	if m.err != nil && m.diff == "" {
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			"",
			m.renderFatalError(),
		)

		return lipgloss.NewStyle().Padding(1, 2).Render(content)
	}

	content := m.renderContent(totalWidth)
	footer := m.renderFooter(totalWidth)

	ui := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		content,
		"",
		"",
		footer,
	)

	return lipgloss.NewStyle().Padding(1, 1).Render(ui)
}

// =============================================================================
// PREFLIGHT
// =============================================================================

func (m Model) renderPreflightScreen(width int) string {
	brand := brandBadgeStyle.Render(" code-review ") + "  " + dimStyle.Render(buildinfo.Version)

	lines := []string{brand, "", sectionStyle.Render("VALIDANDO ENTORNO"), ""}

	if len(m.dependencyResults) == 0 {
		lines = append(
			lines,
			m.spinner.View()+" "+mutedStyle.Render("Comprobando GitHub CLI..."),
		)
	} else {
		for _, dep := range m.dependencyResults {
			icon := successStyle.Render("✓")
			status := mutedStyle.Render("encontrado")

			if !dep.Found {
				icon = errorStyle.Render("✗")
				status = errorStyle.Render("no encontrado")
			}

			lines = append(lines, fmt.Sprintf("%s %s   %s", icon, titleStyle.Render(dep.Label), status))

			if !dep.Found {
				lines = append(lines, "    "+mutedStyle.Render(dep.InstallHint))
			}
		}

		if m.preflight == preflightFailed {
			lines = append(
				lines,
				"",
				errorStyle.Render("Instalá lo que falta y volvé a correr code-review."),
			)
		}
	}

	panel := normalPanel

	if m.preflight == preflightFailed {
		// Sin color de "error": la urgencia se transmite con un borde más
		// grueso, no con un acento cromático.
		panel = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(strongBorder).
			Padding(0, 1)
	}

	return panel.Width(width).Render(strings.Join(lines, "\n"))
}

// =============================================================================
// HEADER
// =============================================================================

func (m Model) renderHeader(width int) string {
	brand := brandStyle.Render("code-review") + "  " + dimStyle.Render(buildinfo.Version)

	if m.prInfo == nil {
		box := normalPanel.Width(width).Render(
			brandStyle.Render(fmt.Sprintf("%s #%d", m.pr.Repository(), m.pr.Number)) +
				"\n\n" +
				m.spinner.View() + " " + mutedStyle.Render(m.loadingText),
		)

		return brand + "\n\n" + box
	}

	line1 := brandStyle.Render(m.prInfo.Title) + " " + dimStyle.Render(fmt.Sprintf("#%d", m.pr.Number))

	line2 := strings.Join([]string{
		prStateStyle.Render(prStateLabel(m.prInfo.State)),
		titleStyle.Render("@" + m.prInfo.Author.Login),
		mutedStyle.Render("wants to merge into"),
		branchStyle.Render(m.prInfo.BaseRefName),
		mutedStyle.Render("from"),
		branchStyle.Render(m.prInfo.HeadRefName),
		mutedStyle.Render("·"),
		mutedStyle.Render(fmt.Sprintf("%d files", m.prInfo.ChangedFiles)),
		mutedStyle.Render("·"),
		successStyle.Render(fmt.Sprintf("+%d", m.prInfo.Additions)),
		errorStyle.Render(fmt.Sprintf("-%d", m.prInfo.Deletions)),
	}, " ")

	prBox := normalPanel.Width(width).Render(line1 + "\n\n" + line2)

	body := brand + "\n\n" + prBox

	if m.diff == "" {
		body += "\n\n"
		body += m.spinner.View() + " "
		body += mutedStyle.Render(m.loadingText)

		return body
	}

	return body
}

// =============================================================================
// SECTION TABS
// =============================================================================

// renderSectionTabs arma la fila horizontal de las 4 categorías con una
// línea debajo de la seleccionada, al estilo de tabs de navegador.
func (m Model) renderSectionTabs() string {
	selected := sectionIndexForStage(m.selectedStage)

	var tabs strings.Builder

	var underlineOffset, underlineWidth int

	for i, sec := range sections() {
		if i > 0 {
			tabs.WriteString("    ")
		}

		count := len(sectionFindings(stagesForSection(m.stages, sec)))
		badge := fmt.Sprintf(" [%d]", count)

		if i == selected {
			underlineOffset = lipgloss.Width(tabs.String())
			underlineWidth = lipgloss.Width(sec.Label + badge)

			tabs.WriteString(selectedStyle.Render(sec.Label))
		} else {
			tabs.WriteString(mutedStyle.Render(sec.Label))
		}

		if count > 0 {
			tabs.WriteString(badgeStyle.Render(badge))
		} else {
			tabs.WriteString(mutedStyle.Render(badge))
		}
	}

	underline := strings.Repeat(" ", underlineOffset) +
		tabIndicatorStyle.Render(strings.Repeat("─", underlineWidth))

	return tabs.String() + "\n" + underline
}

// =============================================================================
// CONTENT
// =============================================================================

// renderContent no usa ninguna caja: el título y subtítulo de la sección,
// el estado (Analyzing/Review complete/Review failed) y el cuerpo respiran
// directamente sobre el fondo, sin borde.
func (m Model) renderContent(width int) string {
	if m.prInfo == nil || m.diff == "" {
		return mutedStyle.Render("Preparing diff...")
	}

	return m.viewport.View()
}

func (m Model) renderStatusLine(status review.Status) string {
	switch status {
	case review.StatusRunning:
		return m.spinner.View() + " " + titleStyle.Render("Analyzing")

	case review.StatusError:
		return errorStyle.Render("✗ Review failed")

	case review.StatusPending:
		return mutedStyle.Render("○ Not started yet")

	default:
		return successStyle.Render("✓ Review complete")
	}
}

// renderSectionBody decide qué mostrar debajo del status: mientras corre,
// una línea de actividad; si el usuario apretó Tab, la salida cruda
// (viewport); si ya terminó, los hallazgos (viewport); todo lo demás sale
// directo del viewport, que refreshViewport ya dejó con el contenido
// correcto para la sección/mode actual.
func (m Model) renderSectionBody(status review.Status) string {
	if status == review.StatusError {
		return m.viewport.View()
	}

	if m.mode == modeClaude {
		return m.viewport.View()
	}

	if status == review.StatusRunning {
		return "  " + mutedStyle.Render(m.activity)
	}

	if status == review.StatusPending {
		return mutedStyle.Render("Esta sección todavía no arrancó.")
	}

	return m.viewport.View()
}

// renderFindingsText arma el documento scrolleable como un diff de GitHub:
// archivos, hunks y comentarios inline con el contexto alrededor.
func (m Model) renderFindingsText(stages []review.Stage, width int) string {
	findings := sectionFindings(stages)
	return renderInlineReview(m.diff, findings, width)
}

// =============================================================================
// FOOTER
// =============================================================================

func (m Model) renderFooter(width int) string {
	rule := lipgloss.NewStyle().Foreground(border).Render(strings.Repeat("─", width))

	controls := []string{
		keyStyle.Render("↑ ↓") + " scroll",
		keyStyle.Render("q") + " quit",
	}

	return rule + "\n\n" + mutedStyle.Render(strings.Join(controls, "   "))
}

// =============================================================================
// ERROR
// =============================================================================

// renderFatalError se usa únicamente para errores previos a que arranque el
// pipeline (no pudimos traer el PR o el diff), donde todavía no hay tabs ni
// secciones que mostrar. Una vez que el pipeline arrancó, el error de una
// sección se ve dentro de renderContent/renderSectionBody.
func (m Model) renderFatalError() string {
	return errorStyle.Render("✗ Review failed") +
		"\n\n" +
		mutedStyle.Render(m.err.Error())
}

// =============================================================================
// HELPERS
// =============================================================================

func wrapText(value string, width int) string {
	if width <= 10 {
		return value
	}

	return lipgloss.NewStyle().Width(width).Render(value)
}

func prStateLabel(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "MERGED":
		return "Merged"
	case "CLOSED":
		return "Closed"
	default:
		return "Open"
	}
}
