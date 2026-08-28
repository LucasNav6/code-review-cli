package ui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/LucasNav6/code-review-cli/internal/claude"
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

func startReviewCmd(stage review.Stage, diff string, stageIndex int) tea.Cmd {
	return func() tea.Msg {
		if !strings.Contains(stage.Prompt, "{{DIFF}}") {
			return claudeFailedMsg{
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
