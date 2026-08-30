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

	commandSummary := m.renderCommandStagesSummary(totalWidth)
	content := m.renderContent(totalWidth)
	loadingBar := m.renderStickyLoadingBar(totalWidth)
	footer := m.renderFooter(totalWidth)

	parts := []string{header}
	if commandSummary != "" {
		parts = append(parts, "", commandSummary)
	}
	if loadingBar != "" {
		parts = append(parts, "", loadingBar)
	}
	parts = append(parts, "", content, "", "", footer)

	ui := lipgloss.JoinVertical(lipgloss.Left, parts...)

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
			m.spinner.View()+" "+mutedStyle.Render("Comprobando GitHub CLI y Claude..."),
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

// renderCommandStagesSummary arma el resumen de las etapas KindCommand
// (SBOM, gitleaks): a diferencia de las etapas que le mandan el diff a
// Claude, estas no apuntan a una línea puntual del PR sino a un chequeo
// general, así que no tiene sentido mostrarlas como comentarios inline.
// Se muestran acá, debajo del header, como un bloque aparte.
func (m Model) renderCommandStagesSummary(width int) string {
	var lines []string

	for _, stage := range m.stages {
		if stage.Kind != review.KindCommand {
			continue
		}

		if line := renderCommandStageLine(stage, width); line != "" {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return ""
	}

	rule := lipgloss.NewStyle().Foreground(border).Render(strings.Repeat("─", width))

	return rule + "\n" + strings.Join(lines, "\n")
}

// renderCommandStageLine devuelve la línea de resumen de una etapa
// KindCommand, o "" mientras todavía no hay nada que mostrar (pendiente o
// corriendo).
func renderCommandStageLine(stage review.Stage, width int) string {
	switch stage.Status {
	case review.StatusError:
		return mutedStyle.Render(fmt.Sprintf("%s error: %s", stage.ShortName, stage.Err.Error()))

	case review.StatusClean:
		return mutedStyle.Render(stage.ShortName + " Not found")

	case review.StatusFindings:
		return renderCommandStageFindings(stage, width)

	default:
		return ""
	}
}

// renderCommandStageFindings arma el bloque "SBOM____________[3]" seguido de
// una línea por hallazgo, para una etapa KindCommand que sí encontró algo.
func renderCommandStageFindings(stage review.Stage, width int) string {
	findings := stage.Findings()

	badge := fmt.Sprintf("[%d]", len(findings))
	fill := width - lipgloss.Width(stage.ShortName) - lipgloss.Width(badge)
	if fill < 1 {
		fill = 1
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render(stage.ShortName + strings.Repeat("_", fill) + badge))

	for _, finding := range findings {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("- " + truncateRunes(commandFindingSummary(finding), width-2)))
	}

	return b.String()
}

// commandFindingSummary arma una línea legible por hallazgo (título si hay,
// si no el comentario), con el archivo entre paréntesis cuando se conoce.
func commandFindingSummary(finding review.Finding) string {
	label := strings.TrimSpace(finding.Title)
	if label == "" {
		label = strings.TrimSpace(finding.Comment)
	}

	if finding.File == "" {
		return label
	}

	return fmt.Sprintf("%s (%s)", label, finding.File)
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

// renderStickyLoadingBar arma el panel fijo de progreso: una línea de
// resumen ("Loading review comments N/total. <detalle>") y, debajo, una
// fila fija por cada etapa mostrando qué está haciendo ahora mismo (o por
// qué se saltea), para que se vea movimiento real del pipeline en paralelo.
func (m Model) renderStickyLoadingBar(width int) string {
	if m.diff == "" || !m.reviewLoading {
		return ""
	}

	rule := lipgloss.NewStyle().Foreground(border).Render(strings.Repeat("─", width))

	done, total := asyncStageProgress(m.stages)
	detail := strings.TrimSpace(m.activity)
	if detail == "" {
		detail = "Claude checks run in parallel · diff stays available"
	}

	headline := m.spinner.View() + " " +
		titleStyle.Render(fmt.Sprintf("Loading review comments %d/%d.", done, total)) +
		" " + mutedStyle.Render(detail)

	lines := []string{rule, headline, rule}

	for _, stage := range m.stages {
		lines = append(lines, m.renderStageActivityLine(stage, width))
	}

	return strings.Join(lines, "\n")
}

// renderStageActivityLine arma la fila fija de una etapa dentro del panel de
// progreso: ícono de estado, nombre corto y el detalle de qué está haciendo
// (o el motivo por el que se salteó, o el resultado final).
func (m Model) renderStageActivityLine(stage review.Stage, width int) string {
	icon := m.stageStatusIcon(stage.Status)
	label := mutedStyle.Render(fmt.Sprintf("%-14s", stage.ShortName))
	detail := mutedStyle.Render(stageActivityDetail(stage))

	return truncateRunes(icon+" "+label+" "+detail, width)
}

func (m Model) stageStatusIcon(status review.Status) string {
	switch status {
	case review.StatusRunning:
		return m.spinner.View()
	case review.StatusClean, review.StatusFindings:
		return successStyle.Render("✓")
	case review.StatusError:
		return errorStyle.Render("✗")
	default:
		return dimStyle.Render("○")
	}
}

// stageActivityDetail decide qué texto mostrar en la fila de una etapa:
// prioriza Activity (texto puntual, seteado por eventos de Claude o por el
// motivo de un skip) y si no hay nada específico, cae a un texto genérico
// según el Status.
func stageActivityDetail(stage review.Stage) string {
	if detail := strings.TrimSpace(stage.Activity); detail != "" {
		return detail
	}

	switch stage.Status {
	case review.StatusRunning:
		return defaultRunningActivity(stage.ShortName)
	case review.StatusClean:
		return "no findings"
	case review.StatusFindings:
		return fmt.Sprintf("%d finding(s)", len(stage.Findings()))
	case review.StatusError:
		if stage.Err != nil {
			return stage.Err.Error()
		}

		return "error"
	default:
		return "pending"
	}
}

func defaultRunningActivity(shortName string) string {
	switch shortName {
	case "SBOM":
		return "scanning dependencies..."
	case "GITLEAKS":
		return "scanning for secrets..."
	default:
		return "reviewing the diff..."
	}
}

func asyncStageProgress(stages []review.Stage) (int, int) {
	var done, total int

	for _, stage := range stages {
		if stage.Kind == review.KindCommand && stage.Status == review.StatusPending {
			continue
		}

		if stage.Kind != review.KindPrompt && stage.Kind != review.KindCommand {
			continue
		}

		total++

		if stage.Status != review.StatusRunning && stage.Status != review.StatusPending {
			done++
		}
	}

	return done, total
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
