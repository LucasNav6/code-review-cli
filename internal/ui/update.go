package ui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/LucasNav6/code-review-cli/internal/depscan"
	"github.com/LucasNav6/code-review-cli/internal/review"
	"github.com/LucasNav6/code-review-cli/internal/secretscan"
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

		m.done = false
		m.executingStage = -1
		m.loadingText = "Loading review comments..."
		m.reviewLoading = true
		m.markAsyncStagesRunning(depscan.ShouldScanDiff(m.diff))

		m.refreshViewport()

		baseRefName := ""
		if m.prInfo != nil {
			baseRefName = m.prInfo.BaseRefName
		}

		return m, startAsyncReviewsCmd(m.pr, m.headSHA, baseRefName, m.diff)

	case claudeStartedMsg:
		m.claudeChannel = msg.channel

		return m, waitForClaudeEvent(msg.stage, msg.channel)

	case claudeChunkMsg:
		if msg.stage >= 0 && msg.stage < len(m.stages) {
			m.stages[msg.stage].RawOutput += msg.text
		}

		return m, waitForClaudeEvent(msg.stage, msg.channel)

	case claudeStatusMsg:
		m.activity = msg.text

		if msg.stage >= 0 && msg.stage < len(m.stages) {
			m.stages[msg.stage].Activity = msg.text
		}

		return m, waitForClaudeEvent(msg.stage, msg.channel)

	case claudeFinishedMsg:
		return m.finishPromptReview(msg.stage, msg.result)

	case commandFinishedMsg:
		return m.applyParallelStageResult(msg.stage, msg.result, msg.rawOutput)

	case stageFailedMsg:
		if msg.stage >= 0 && msg.stage < len(m.stages) {
			m.stages[msg.stage].Status = review.StatusError
			m.stages[msg.stage].Err = msg.err
		}

		m.activity = msg.err.Error()
		m.finishReviewIfAllAsyncStagesDone()

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
	}

	// Todo lo demás (↑ ↓ PgUp PgDn, etc.) scrollea el diff.
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
	contentWidth := m.width - 2

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

func (m *Model) markAsyncStagesRunning(includeDependencies bool) {
	for i := range m.stages {
		switch {
		case m.stages[i].Kind == review.KindPrompt:
			m.stages[i].Status = review.StatusRunning

		case m.stages[i].ShortName == "SBOM":
			if includeDependencies {
				m.stages[i].Status = review.StatusRunning
			} else {
				m.stages[i].Activity = "skipped — no dependency changes in this diff"
			}

		case m.stages[i].ShortName == "GITLEAKS":
			if secretscan.Available() {
				m.stages[i].Status = review.StatusRunning
			} else {
				m.stages[i].Activity = "skipped — gitleaks is not installed"
			}
		}
	}
}

// refreshViewport recalcula el contenido scrolleable principal.
func (m *Model) refreshViewport() {
	if m.err != nil {
		detail := ""

		if m.err != nil {
			detail = m.err.Error()
		}

		m.viewport.SetContent(wrapText(mutedStyle.Render(detail), m.viewport.Width()))
		m.viewport.GotoTop()

		return
	}

	m.viewport.SetContent(renderFilesChanged(m.diff, allFindings(m.stages), m.viewport.Width()))
	m.viewport.GotoTop()
}

func (m *Model) finishReviewIfAllAsyncStagesDone() {
	for _, stage := range m.stages {
		if (stage.Kind == review.KindPrompt || stage.Kind == review.KindCommand) &&
			stage.Status == review.StatusRunning {
			return
		}
	}

	m.done = true
	m.reviewLoading = false
	m.loadingText = "Review comments loaded."
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

	normalizeStageFindings(result, stage.ShortName)

	return m.applyParallelStageResult(stageIndex, result, rawResult)
}

func normalizeStageFindings(result *review.Result, category string) {
	if result == nil {
		return
	}

	for i := range result.Findings {
		if strings.TrimSpace(result.Findings[i].Category) == "" {
			result.Findings[i].Category = category
		}
	}
}

func (m Model) applyParallelStageResult(stageIndex int, result *review.Result, rawOutput string) (tea.Model, tea.Cmd) {
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

	m.finishReviewIfAllAsyncStagesDone()
	m.refreshViewport()

	return m, nil
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
