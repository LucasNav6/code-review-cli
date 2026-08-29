package ui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LucasNav6/code-review-cli/internal/claude"
	"github.com/LucasNav6/code-review-cli/internal/depscan"
	"github.com/LucasNav6/code-review-cli/internal/githubpr"
	"github.com/LucasNav6/code-review-cli/internal/review"
)

func fetchPRInfoCmd(pr githubpr.PullRequest) tea.Cmd {
	return func() tea.Msg {
		info, err := githubpr.FetchInfo(pr)
		if err != nil {
			return appErrorMsg{err: err}
		}

		return prInfoLoadedMsg{info: *info}
	}
}

func fetchDiffCmd(pr githubpr.PullRequest, outputPath string) tea.Cmd {
	return func() tea.Msg {
		diff, err := githubpr.FetchDiff(pr)
		if err != nil {
			return appErrorMsg{err: err}
		}

		if err := os.WriteFile(outputPath, []byte(diff), 0o644); err != nil {
			return appErrorMsg{
				err: fmt.Errorf("obtuve el diff pero no pude guardar %s: %w", outputPath, err),
			}
		}

		return diffLoadedMsg{diff: diff}
	}
}

// startReviewCmd arranca la etapa indicada, sin importar de qué tipo sea:
// las etapas review.KindPrompt corren vía Claude, las review.KindCommand
// corren un chequeo aislado por comandos (sin ningún LLM de por medio).
func startReviewCmd(pr githubpr.PullRequest, headSHA string, stage review.Stage, diff string, stageIndex int) tea.Cmd {
	if stage.Kind == review.KindCommand {
		return runDependencyScanCmd(pr, headSHA, stageIndex)
	}

	return startPromptReviewCmd(stage, diff, stageIndex)
}

func startPromptReviewCmd(stage review.Stage, diff string, stageIndex int) tea.Cmd {
	return func() tea.Msg {
		if !strings.Contains(stage.Prompt, "{{DIFF}}") {
			return stageFailedMsg{
				stage: stageIndex,
				err:   fmt.Errorf("el prompt de %s no contiene {{DIFF}}", stage.Name),
			}
		}

		prompt := strings.Replace(stage.Prompt, "{{DIFF}}", diff, 1)

		channel := claude.Stream(prompt)

		return claudeStartedMsg{
			stage:   stageIndex,
			channel: channel,
		}
	}
}

func startAsyncReviewsCmd(pr githubpr.PullRequest, headSHA string, diff string) tea.Cmd {
	stages := review.DefaultStages()
	commands := make([]tea.Cmd, 0, len(stages))

	for i, stage := range stages {
		switch {
		case stage.Kind == review.KindPrompt:
			commands = append(commands, startPromptReviewCmd(stage, diff, i))

		case stage.Kind == review.KindCommand && depscan.ShouldScanDiff(diff):
			commands = append(commands, runDependencyScanCmd(pr, headSHA, i))
		}
	}

	return tea.Batch(commands...)
}

// runDependencyScanCmd corre el escaneo de dependencias con OSV-Scanner.
// No es incremental (no hay streaming): corre, y devuelve el resultado ya
// armado de una sola vez.
func runDependencyScanCmd(pr githubpr.PullRequest, headSHA string, stageIndex int) tea.Cmd {
	return func() tea.Msg {
		result, rawOutput, err := depscan.Scan(pr, headSHA)
		if err != nil {
			return stageFailedMsg{stage: stageIndex, err: err}
		}

		return commandFinishedMsg{
			stage:     stageIndex,
			result:    result,
			rawOutput: rawOutput,
		}
	}
}
