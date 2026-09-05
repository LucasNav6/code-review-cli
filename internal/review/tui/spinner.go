package tui

import (
	"charm.land/bubbletea/v2"

	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
)

// ReviewReadyMsg is the bubbletea Msg the cmd layer sends when
// the use case has finished. The model transitions from
// AnalysisRunning to AnalysisDone or AnalysisError depending on
// whether Err is nil.
//
// We declare it here (not in model.go) because ReviewReadyMsg is
// the public contract the cmd layer depends on; keeping it in
// the same file as the spinner keeps all the "what the cmd does"
// pieces together.
type ReviewReadyMsg struct {
	Review reviewdomain.Review
	Err    error
}

// runningMsg is bubbletea's TickMsg for the spinner. We forward
// any TickMsg into the spinner so it animates. We use a named
// type alias so future custom tick types (e.g. per-category
// progress) can layer on top without breaking the spinner
// update path.
type runningMsg = tea.Msg

// ensure tea is referenced (the alias needs the import even
// though we never declare a value of runningMsg in production
// code).
var _ runningMsg
