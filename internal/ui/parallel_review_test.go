package ui

import (
	"testing"

	"github.com/LucasNav6/code-review-cli/internal/githubpr"
	"github.com/LucasNav6/code-review-cli/internal/review"
)

func TestPromptStageProgressIgnoresCommandStages(t *testing.T) {
	stages := review.DefaultStages()

	for i := range stages {
		if stages[i].Kind == review.KindPrompt {
			stages[i].Status = review.StatusRunning
		}
	}

	stages[0].Status = review.StatusClean
	stages[1].Status = review.StatusPending

	done, total := promptStageProgress(stages)

	if done != 1 || total != 4 {
		t.Fatalf("expected progress 1/4, got %d/%d", done, total)
	}
}

func TestFinishReviewIfAllPromptStagesDoneIgnoresCommandStages(t *testing.T) {
	model := New(*testPullRequest())
	model.stages = review.DefaultStages()
	model.reviewLoading = true

	for i := range model.stages {
		if model.stages[i].Kind == review.KindPrompt {
			model.stages[i].Status = review.StatusClean
		}
	}

	model.stages[1].Status = review.StatusPending
	model.finishReviewIfAllPromptStagesDone()

	if !model.done {
		t.Fatal("expected model to be done when all prompt stages are done")
	}

	if model.reviewLoading {
		t.Fatal("expected loading state to stop")
	}
}

func testPullRequest() *githubpr.PullRequest {
	return githubpr.New("org", "repo", 1)
}
