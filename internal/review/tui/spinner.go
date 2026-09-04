package tui

import (
	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"

	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
)

// NewRunning creates a Model in the running state. The overview
// view paints a spinner + the supplied categoryHint (e.g.
// "Fetching diff" / "Running security review" / etc.) so the
// user sees progress during the long phases of the use case.
//
// The model is wired to receive a ReviewReadyMsg that flips the
// analysis state to AnalysisDone and replaces the spinner with
// the final Review. The cmd layer (F7) sends this message after
// the use case returns.
//
// Usage:
//
//	model := tui.NewRunning("Fetching diff")
//	p := tea.NewProgram(model)
//	go func() {
//	    review, err := useCase.Execute(ctx, input)
//	    p.Send(tui.ReviewReadyMsg{Review: review, Err: err})
//	}()
//	p.Run()
func NewRunning(categoryHint string) *Model {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	return &Model{
		view:          ViewOverview,
		analysis:      AnalysisRunning,
		spinner:       s,
		categoryHint:  categoryHint,
		statusMessage: categoryHint,
	}
}

// ReviewReadyMsg is the bubbletea Msg the cmd layer sends when
// the use case has finished. The model transitions from
// AnalysisRunning to AnalysisDone or AnalysisError depending on
// whether Err is nil.
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

// spinnerStyle colours the spinner glyph to match the overview's
// muted palette so the running screen feels like the same UI.
var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E"))

// ensure spinner is referenced (used by NewRunning above; kept
// the import explicit here so a future refactor that swaps the
// spinner implementation cannot accidentally drop it).
var _ spinner.Model = spinner.New()