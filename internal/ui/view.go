package ui

import (
	"fmt"
	"strconv"
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

	totalWidth := max(70, m.width-4)

	if m.preflight != preflightPassed {
		return lipgloss.NewStyle().
			Padding(1, 2).
			Render(m.renderPreflightScreen(totalWidth))
	}

	header := m.renderHeader(totalWidth)

	if m.err != nil {
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			"",
			m.renderError(totalWidth),
		)

		return lipgloss.NewStyle().Padding(1, 2).Render(content)
	}

	asideWidth := int(float64(totalWidth) * 0.27)

	if asideWidth < 27 {
		asideWidth = 27
	}

	contentWidth := totalWidth - asideWidth - 1

	aside := m.renderAside(asideWidth)
	content := m.renderContent(contentWidth)

	body := lipgloss.JoinHorizontal(lipgloss.Top, aside, " ", content)

	footer := m.renderFooter()

	ui := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		body,
		"",
		footer,
	)

	return lipgloss.NewStyle().Padding(1, 2).Render(ui)
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
			m.spinner.View()+" "+mutedStyle.Render("Comprobando que tengas gh y claude instalados..."),
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
		panel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(danger).
			Padding(0, 1)
	}

	return panel.Width(width).Render(strings.Join(lines, "\n"))
}

// =============================================================================
// HEADER
// =============================================================================

func (m Model) renderHeader(width int) string {
	brand := brandBadgeStyle.Render(" code-review ") + "  " + dimStyle.Render(buildinfo.Version)

	if m.prInfo == nil {
		body := brand + "\n\n"
		body += brandStyle.Render(fmt.Sprintf("%s #%d", m.pr.Repository(), m.pr.Number))
		body += "\n\n"
		body += m.spinner.View() + " "
		body += mutedStyle.Render(m.loadingText)

		return normalPanel.Width(width).Render(body)
	}

	line1 := fmt.Sprintf(
		"%s %s",
		brandStyle.Render(fmt.Sprintf("#%d", m.pr.Number)),
		titleStyle.Render(m.prInfo.Title),
	)

	line2 := fmt.Sprintf(
		"%s  %s",
		titleStyle.Render(m.pr.Repository()),
		mutedStyle.Render("@"+m.prInfo.Author.Login),
	)

	line3 := fmt.Sprintf(
		"%s %s %s   %s   %s   %s",
		infoStyle.Render(m.prInfo.HeadRefName),
		mutedStyle.Render("→"),
		infoStyle.Render(m.prInfo.BaseRefName),
		mutedStyle.Render(fmt.Sprintf("%d archivos", m.prInfo.ChangedFiles)),
		successStyle.Render(fmt.Sprintf("+%d", m.prInfo.Additions)),
		errorStyle.Render(fmt.Sprintf("-%d", m.prInfo.Deletions)),
	)

	body := brand + "\n\n" + strings.Join([]string{line1, line2, "", line3}, "\n")

	if m.diff == "" {
		body += "\n\n"
		body += m.spinner.View() + " "
		body += mutedStyle.Render(m.loadingText)
	}

	return normalPanel.Width(width).Render(body)
}

// =============================================================================
// ASIDE
// =============================================================================

func (m Model) renderAside(width int) string {
	var lines []string

	lines = append(lines, sectionStyle.Render("REVISIONES"), "")

	for i, stage := range m.stages {
		selected := i == m.selectedStage

		icon := m.stageIcon(stage)

		name := stage.Name

		if selected {
			name = selectedStyle.Render(name)
		} else {
			name = titleStyle.Render(name)
		}

		count := ""

		if stage.Result != nil {
			count = strconv.Itoa(len(stage.Result.Findings))
		}

		if stage.Status == review.StatusRunning {
			count = "..."
		}

		row := fmt.Sprintf("%s %s", icon, name)

		if count != "" {
			row += " " + mutedStyle.Render("["+count+"]")
		}

		if selected {
			row = selectedStyle.Render("▸") + " " + row
		} else {
			row = "  " + row
		}

		lines = append(lines, row)
		lines = append(lines, "    "+mutedStyle.Render(stage.Description), "")
	}

	style := normalPanel

	if m.selectedStage >= 0 {
		style = activePanel
	}

	return style.Width(width).Render(strings.Join(lines, "\n"))
}

func (m Model) stageIcon(stage review.Stage) string {
	switch stage.Status {
	case review.StatusRunning:
		return m.spinner.View()

	case review.StatusClean:
		return successStyle.Render("✓")

	case review.StatusFindings:
		return warningStyle.Render("!")

	case review.StatusError:
		return errorStyle.Render("✗")

	default:
		return mutedStyle.Render("○")
	}
}

// =============================================================================
// CONTENT
// =============================================================================

func (m Model) renderContent(width int) string {
	if m.prInfo == nil || m.diff == "" {
		return normalPanel.Width(width).Render(mutedStyle.Render("Preparando el análisis..."))
	}

	stage := m.stages[m.selectedStage]

	header := m.renderContentHeader(stage)

	var content string

	if stage.Status == review.StatusRunning && stage.Result == nil {
		content = m.renderClaudeContent(stage)
	} else if m.mode == modeClaude {
		content = m.renderClaudeContent(stage)
	} else {
		content = m.renderFindingContent(stage, width)
	}

	body := header + "\n\n" + content

	return activePanel.Width(width).Render(body)
}

func (m Model) renderContentHeader(stage review.Stage) string {
	var mode string

	if stage.Status == review.StatusRunning {
		mode = m.spinner.View() + " ANALIZANDO"
	} else if m.mode == modeClaude {
		mode = "CLAUDE OUTPUT"
	} else {
		mode = "HALLAZGOS"
	}

	return fmt.Sprintf("%s    %s", sectionStyle.Render(stage.Name), mutedStyle.Render(mode))
}

// =============================================================================
// FINDING CONTENT
// =============================================================================

func (m Model) renderFindingContent(stage review.Stage, width int) string {
	if stage.Status == review.StatusPending {
		return mutedStyle.Render("Esta revisión todavía no comenzó.")
	}

	if stage.Status == review.StatusError {
		return errorStyle.Render("No pude completar esta revisión.")
	}

	if stage.Result == nil {
		return mutedStyle.Render("No hay un resultado disponible.")
	}

	if len(stage.Result.Findings) == 0 {
		body := successStyle.Render("✓ Sin observaciones")

		if stage.Result.Summary != "" {
			body += "\n\n" + mutedStyle.Render(stage.Result.Summary)
		}

		return body
	}

	index := stage.SelectedFinding

	if index < 0 {
		index = 0
	}

	if index >= len(stage.Result.Findings) {
		index = len(stage.Result.Findings) - 1
	}

	finding := stage.Result.Findings[index]

	var body strings.Builder

	body.WriteString(categoryStyle.Render(finding.Category))
	body.WriteString("\n")
	body.WriteString(mutedStyle.Render(fmt.Sprintf("Hallazgo %d de %d", index+1, len(stage.Result.Findings))))
	body.WriteString("\n\n")

	location := finding.File

	if finding.Line > 0 {
		location += ":" + strconv.Itoa(finding.Line)
	}

	body.WriteString(fileStyle.Render(location))
	body.WriteString("\n\n")

	if finding.Title != "" {
		body.WriteString(titleStyle.Render(finding.Title))
		body.WriteString("\n\n")
	}

	for _, detail := range finding.Details {
		if detail.Value == "" {
			continue
		}

		body.WriteString(mutedStyle.Render(detail.Label + ":"))
		body.WriteString("\n")
		body.WriteString(wrapText(detail.Value, width-8))
		body.WriteString("\n\n")
	}

	if finding.Comment != "" {
		body.WriteString(titleStyle.Render("Comentario"))
		body.WriteString("\n")
		body.WriteString(wrapText(finding.Comment, width-8))
		body.WriteString("\n\n")
	}

	if finding.Suggestion != "" {
		body.WriteString(titleStyle.Render("Sugerencia"))
		body.WriteString("\n")
		body.WriteString(wrapText(finding.Suggestion, width-8))
	}

	return body.String()
}

// =============================================================================
// CLAUDE OUTPUT
// =============================================================================

func (m Model) renderClaudeContent(stage review.Stage) string {
	if stage.RawOutput == "" {
		if stage.Status == review.StatusPending {
			return mutedStyle.Render("Claude todavía no ejecutó esta revisión.")
		}

		return m.spinner.View() + " " + mutedStyle.Render("Esperando la respuesta de Claude...")
	}

	return m.viewport.View()
}

// =============================================================================
// FOOTER
// =============================================================================

func (m Model) renderFooter() string {
	mode := "Hallazgos"

	if m.mode == modeClaude {
		mode = "Claude"
	}

	var controls []string

	controls = append(controls, keyStyle.Render("← →")+" revisión")

	if m.mode == modeClaude {
		controls = append(
			controls,
			keyStyle.Render("↑ ↓")+" scroll",
			keyStyle.Render("PgUp PgDn")+" página",
		)
	} else {
		controls = append(controls, keyStyle.Render("↑ ↓")+" hallazgo")
	}

	controls = append(
		controls,
		keyStyle.Render("Tab")+" "+mode,
		keyStyle.Render("q")+" salir",
	)

	return mutedStyle.Render(strings.Join(controls, "   "))
}

// =============================================================================
// ERROR
// =============================================================================

func (m Model) renderError(width int) string {
	return normalPanel.Width(width).Render(
		errorStyle.Bold(true).Render("✗ No pude completar la revisión") +
			"\n\n" +
			errorStyle.Render(m.err.Error()),
	)
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
