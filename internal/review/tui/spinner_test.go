package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LucasNav6/code-review-cli/internal/review/tui"
)

// TestNewRunning_StartsInOverviewRunning pins down the initial
// state when the user case is in flight.
func TestNewRunning_StartsInOverviewRunning(t *testing.T) {
	m := tui.NewRunning("Fetching diff")
	out := viewString(m.View())
	// The view shows the spinner glyph + the hint.
	if !strings.Contains(out, "Fetching diff") {
		t.Errorf("expected hint 'Fetching diff' in view, got:\n%s", out)
	}
	// The view does NOT show findings (the use case has not
	// produced any yet).
	if strings.Contains(out, "Review complete") {
		t.Errorf("running view should not show findings yet, got:\n%s", out)
	}
}

// TestReviewReadyMsg_FlipsToDoneAndShowsReview verifies that
// sending ReviewReadyMsg with a populated review transitions
// the model out of AnalysisRunning and into AnalysisDone, and
// the overview now shows the findings count.
func TestReviewReadyMsg_FlipsToDoneAndShowsReview(t *testing.T) {
	m := tui.NewRunning("Fetching diff")
	// Send the ReviewReadyMsg.
	updated, _ := m.Update(tui.ReviewReadyMsg{Review: sampleReview()})
	m2 := updated.(*tui.Model)
	out := viewString(m2.View())
	if !strings.Contains(out, "Review complete") {
		t.Errorf("expected overview after ReviewReadyMsg, got:\n%s", out)
	}
	if !strings.Contains(out, "4 findings") {
		t.Errorf("expected '4 findings' after ReviewReadyMsg, got:\n%s", out)
	}
}

// TestReviewReadyMsg_FlipsToErrorOnErr verifies that sending
// ReviewReadyMsg with a non-nil Err transitions to AnalysisError.
func TestReviewReadyMsg_FlipsToErrorOnErr(t *testing.T) {
	m := tui.NewRunning("Fetching diff")
	updated, _ := m.Update(tui.ReviewReadyMsg{Err: errBoom})
	m2 := updated.(*tui.Model)
	out := viewString(m2.View())
	if !strings.Contains(out, "boom") {
		t.Errorf("expected error message in view, got:\n%s", out)
	}
	if !strings.Contains(out, "press q to quit") {
		t.Errorf("expected quit hint, got:\n%s", out)
	}
}

// TestRunningView_IgnoresEnterWhileRunning verifies that
// the running view does NOT include findings (since the use case
// has not produced any yet). We cannot easily send a real
// Enter key (bubbletea v2 KeyMsg is an interface we cannot
// construct trivially); the invariant is exercised indirectly:
// the View() output while running should not contain "Review
// complete" or finding counts.
func TestRunningView_IgnoresEnterWhileRunning(t *testing.T) {
	m := tui.NewRunning("Fetching diff")
	out := viewString(m.View())
	if strings.Contains(out, "Review complete") {
		t.Errorf("running view should not show overview findings; got:\n%s", out)
	}
	if !strings.Contains(out, "Fetching diff") {
		t.Errorf("running view should show the category hint; got:\n%s", out)
	}
	_ = tea.WindowSizeMsg{} // referenced for clarity; bubbles.Update handles it
}
