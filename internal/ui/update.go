package ui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/LucasNav6/code-review-cli/internal/review"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()

		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd

		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd

	case preflightMsg:
		m.dependencyResults = msg.results

		if !allDependenciesFound(msg.results) {
			m.preflight = preflightFailed
			return m, nil
		}

		m.preflight = preflightPassed
		m.loadingText = "Buscando la información del pull request..."

		return m, fetchPRInfoCmd(m.pr)

	case prInfoLoadedMsg:
		m.prInfo = &msg.info
		m.headSHA = msg.info.HeadRefOid

		m.loadingText = "Obteniendo los archivos y el código modificado del pull request..."

		return m, fetchDiffCmd(m.pr, m.diffPath)

	case diffLoadedMsg:
		m.diff = msg.diff

		m.executingStage = 0
		m.selectedStage = 0

		m.stages[0].Status = review.StatusRunning

		m.activity = "Reviewing changed files..."

		m.refreshViewport()

		return m, startReviewCmd(m.pr, m.headSHA, m.stages[0], m.diff, 0)

	case claudeStartedMsg:
		m.claudeChannel = msg.channel

		return m, waitForClaudeEvent(msg.stage, msg.channel)

	case claudeChunkMsg:
		if msg.stage >= 0 && msg.stage < len(m.stages) {
			m.stages[msg.stage].RawOutput += msg.text

			if m.mode == modeClaude && sectionIndexForStage(msg.stage) == sectionIndexForStage(m.selectedStage) {
				m.refreshViewport()
			}
		}

		return m, waitForClaudeEvent(msg.stage, m.claudeChannel)

	case claudeStatusMsg:
		m.activity = msg.text

		return m, waitForClaudeEvent(msg.stage, m.claudeChannel)

	case claudeFinishedMsg:
		return m.finishPromptReview(msg.stage, msg.result)

	case commandFinishedMsg:
		return m.applyStageResult(msg.stage, msg.result, msg.rawOutput)

	case stageFailedMsg:
		if msg.stage >= 0 && msg.stage < len(m.stages) {
			m.stages[msg.stage].Status = review.StatusError
		}

		m.err = msg.err

		m.refreshViewport()

		return m, nil

	case appErrorMsg:
		m.err = msg.err

		return m, nil
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "left", "h":
		m.selectPreviousSection()
		return m, nil

	case "right", "l":
		m.selectNextSection()
		return m, nil

	case "tab":
		m.toggleMode()
		return m, nil
	}

	// Todo lo demás (↑ ↓ PgUp PgDn, etc.) scrollea el contenido de la
	// sección actual, sea la lista de hallazgos o la salida cruda.
	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)

	return m, cmd
}

func (m *Model) selectPreviousSection() {
	current := sectionIndexForStage(m.selectedStage)

	if current > 0 {
		m.selectedStage = sections()[current-1].StageIndices[0]
	}

	m.refreshViewport()
}

func (m *Model) selectNextSection() {
	current := sectionIndexForStage(m.selectedStage)

	if current < len(sections())-1 {
		m.selectedStage = sections()[current+1].StageIndices[0]
	}

	m.refreshViewport()
}

func (m *Model) toggleMode() {
	if m.mode == modeFindings {
		m.mode = modeClaude
	} else {
		m.mode = modeFindings
	}

	m.refreshViewport()
}

func (m *Model) resizeViewport() {
	contentWidth := m.width - 10

	if contentWidth < 30 {
		contentWidth = 30
	}

	contentHeight := m.height - 15

	if contentHeight < 8 {
		contentHeight = 8
	}

	m.viewport.SetWidth(contentWidth)
	m.viewport.SetHeight(contentHeight)

	m.refreshViewport()
}

// refreshViewport recalcula el contenido scrolleable de la sección
// actualmente seleccionada según su estado y el modo (hallazgos vs salida
// cruda). El status y la línea de actividad mientras corre se renderizan
// aparte, directamente en view.go, porque son una sola línea fija.
func (m *Model) refreshViewport() {
	stages := m.currentSectionStages()
	status := sectionStatus(stages)

	switch {
	case status == review.StatusError:
		detail := ""

		if m.err != nil {
			detail = m.err.Error()
		}

		m.viewport.SetContent(wrapText(mutedStyle.Render(detail), m.viewport.Width()))
		m.viewport.GotoTop()

	case m.mode == modeClaude:
		content := sectionRawOutput(stages)

		if content == "" {
			content = mutedStyle.Render("Todavía no hay salida.")
		}

		m.viewport.SetContent(content)

		if status == review.StatusRunning {
			m.viewport.GotoBottom()
		} else {
			m.viewport.GotoTop()
		}

	case status == review.StatusRunning, status == review.StatusPending:
		m.viewport.SetContent("")

	default:
		m.viewport.SetContent(m.renderFindingsText(stages, m.viewport.Width()))
		m.viewport.GotoTop()
	}
}

// finishPromptReview interpreta la respuesta cruda de Claude para una etapa
// review.KindPrompt y delega el resto (guardar resultado, avanzar de etapa)
// en applyStageResult.
func (m Model) finishPromptReview(stageIndex int, rawResult string) (tea.Model, tea.Cmd) {
	if stageIndex < 0 || stageIndex >= len(m.stages) {
		return m, nil
	}

	stage := m.stages[stageIndex]

	result, err := review.ParseResult(rawResult, stage.ShortName)

	if err != nil {
		// Importante: no abortamos todo el pipeline por una respuesta
		// inesperada. La convertimos en un hallazgo visible para poder
		// inspeccionarla.
		result = review.FallbackResult(rawResult, stage.ShortName)
	}

	return m.applyStageResult(stageIndex, result, rawResult)
}

// applyStageResult guarda el resultado ya estructurado de una etapa (venga
// de Claude ya parseado, o directo de un chequeo por comandos) y avanza el
// pipeline a la siguiente etapa.
func (m Model) applyStageResult(stageIndex int, result *review.Result, rawOutput string) (tea.Model, tea.Cmd) {
	if stageIndex < 0 || stageIndex >= len(m.stages) {
		return m, nil
	}

	stage := &m.stages[stageIndex]

	stage.Result = result

	if strings.TrimSpace(rawOutput) != "" {
		stage.RawOutput = rawOutput
	}

	if len(result.Findings) == 0 {
		stage.Status = review.StatusClean

		if err := removeOldOutput(stage.OutputPath); err != nil {
			m.err = err
			return m, nil
		}
	} else {
		stage.Status = review.StatusFindings

		markdown := review.RenderMarkdown(stage.Name, result)

		if err := os.WriteFile(stage.OutputPath, []byte(markdown), 0o644); err != nil {
			m.err = fmt.Errorf("no pude crear %s: %w", stage.OutputPath, err)
			return m, nil
		}
	}

	next := stageIndex + 1

	if next >= len(m.stages) {
		m.done = true
		m.executingStage = -1

		for i := range m.stages {
			if m.stages[i].Status == review.StatusFindings {
				m.selectedStage = i
				break
			}
		}

		m.refreshViewport()

		return m, nil
	}

	m.executingStage = next
	m.selectedStage = next

	m.stages[next].Status = review.StatusRunning

	m.activity = fmt.Sprintf("Running %s...", m.stages[next].Name)

	m.refreshViewport()

	return m, startReviewCmd(m.pr, m.headSHA, m.stages[next], m.diff, next)
}

func removeOldOutput(path string) error {
	err := os.Remove(path)

	if err == nil || os.IsNotExist(err) {
		return nil
	}

	return fmt.Errorf("no pude eliminar %s: %w", path, err)
}
