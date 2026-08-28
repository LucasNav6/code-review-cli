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
		m.mode = modeClaude

		m.stages[0].Status = review.StatusRunning

		m.activity = "Claude está revisando seguridad según OWASP API Top 10 2023..."

		m.refreshViewport()

		return m, startReviewCmd(m.pr, m.headSHA, m.stages[0], m.diff, 0)

	case claudeStartedMsg:
		m.claudeChannel = msg.channel

		return m, waitForClaudeEvent(msg.stage, msg.channel)

	case claudeChunkMsg:
		if msg.stage >= 0 && msg.stage < len(m.stages) {
			m.stages[msg.stage].RawOutput += msg.text

			if m.selectedStage == msg.stage && m.mode == modeClaude {
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
		m.selectPreviousStage()
		return m, nil

	case "right", "l":
		m.selectNextStage()
		return m, nil

	case "tab":
		m.toggleMode()
		return m, nil
	}

	if m.mode == modeClaude {
		var cmd tea.Cmd

		m.viewport, cmd = m.viewport.Update(msg)

		return m, cmd
	}

	stage := &m.stages[m.selectedStage]

	switch key {
	case "up", "k":
		if len(stage.Findings()) > 0 && stage.SelectedFinding > 0 {
			stage.SelectedFinding--
		}

	case "down", "j":
		if len(stage.Findings()) > 0 && stage.SelectedFinding < len(stage.Findings())-1 {
			stage.SelectedFinding++
		}

	case "home", "g":
		stage.SelectedFinding = 0

	case "end", "G":
		if len(stage.Findings()) > 0 {
			stage.SelectedFinding = len(stage.Findings()) - 1
		}
	}

	return m, nil
}

func (m *Model) selectPreviousStage() {
	if m.selectedStage > 0 {
		m.selectedStage--
	}

	m.normalizeSelectedFinding()
	m.refreshViewport()
}

func (m *Model) selectNextStage() {
	if m.selectedStage < len(m.stages)-1 {
		m.selectedStage++
	}

	m.normalizeSelectedFinding()
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

func (m *Model) normalizeSelectedFinding() {
	stage := &m.stages[m.selectedStage]

	count := len(stage.Findings())

	if count == 0 {
		stage.SelectedFinding = 0
		return
	}

	if stage.SelectedFinding >= count {
		stage.SelectedFinding = count - 1
	}
}

func (m *Model) resizeViewport() {
	asideWidth := int(float64(m.width-4) * 0.27)

	contentWidth := m.width - asideWidth - 10

	if contentWidth < 30 {
		contentWidth = 30
	}

	contentHeight := m.height - 17

	if contentHeight < 8 {
		contentHeight = 8
	}

	m.viewport.SetWidth(contentWidth)
	m.viewport.SetHeight(contentHeight)

	m.refreshViewport()
}

func (m *Model) refreshViewport() {
	if m.selectedStage < 0 || m.selectedStage >= len(m.stages) {
		return
	}

	stage := m.stages[m.selectedStage]

	content := stage.RawOutput

	if content == "" {
		content = "Esperando la respuesta de Claude..."
	}

	m.viewport.SetContent(content)

	if stage.Status == review.StatusRunning {
		m.viewport.GotoBottom()
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
				m.mode = modeFindings
				break
			}
		}

		m.refreshViewport()

		return m, nil
	}

	m.executingStage = next
	m.selectedStage = next

	m.stages[next].Status = review.StatusRunning

	m.mode = modeClaude

	m.activity = "Ejecutando " + m.stages[next].Name + "..."

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
