// Package tui implements the bubbletea-based interactive review UI.
// The TUI consumes a domain.Review produced by the use case and
// lets the user navigate the findings in a three-view flow:
//
//   overview → findings list → finding detail
//
// F4 ships the model + overview view. F5 adds the findings list
// (bubbles/list) and finding detail (bubbles/viewport). F6 wires
// the spinner for the analysis phase. F7 plugs the TUI into cmd.
package tui

import (
	"charm.land/bubbletea/v2"

	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
)

// View identifies which screen the TUI is currently showing.
type View int

const (
	// ViewOverview is the first screen the user sees after
	// the review completes. It shows PR metadata + severity +
	// category counts + a top-N list.
	ViewOverview View = iota

	// ViewFindings is the navigable list of findings (added in
	// F5).
	ViewFindings

	// ViewDetail is the per-finding detail screen (added in F5).
	ViewDetail
)

// String returns a stable, human-readable identifier for a View.
// Useful in tests and in any debug logging we add later.
func (v View) String() string {
	switch v {
	case ViewOverview:
		return "overview"
	case ViewFindings:
		return "findings"
	case ViewDetail:
		return "detail"
	default:
		return "unknown"
	}
}

// AnalysisState tracks whether the use case is currently running
// (F6) or has finished (F4).
type AnalysisState int

const (
	// AnalysisIdle is the state before the user invokes
	// 'code-review review --url <pr>'. Today the TUI never sees
	// this because the cmd layer only constructs the program
	// after the use case has returned.
	AnalysisIdle AnalysisState = iota

	// AnalysisRunning is the state while the use case is fetching
	// diff + metadata + running LLM passes. The spinner + status
	// messages are shown on the overview screen.
	AnalysisRunning

	// AnalysisDone is the state after the use case has returned.
	// The overview switches from "running…" to the final
	// counts.
	AnalysisDone

	// AnalysisError is the state when the use case returned an
	// error. The TUI shows the error message and lets the user
	// quit.
	AnalysisError
)

// Model is the bubbletea tea.Model. F4 ships the overview view;
// F5 will add fields for the findings list + viewport.
//
// The model is intentionally a single struct (not a tree of
// sub-models) because the TUI is small (3 views) and the cost of
// a tree-of-models with bubbletea.Update composition is not
// worth it for this size. When the TUI grows (multi-PR review
// history, settings panel, etc.) this should be revisited.
type Model struct {
	// width / height track the terminal size. bubbletea sends a
	// WindowSizeMsg whenever the terminal is resized; we store
	// the values so the View functions can use them.
	width  int
	height int

	// view is the current screen. Switching is done by setting
	// this field + returning the matching Cmd.
	view View

	// analysis tracks whether the use case is still running.
	// F4 only reads AnalysisDone / AnalysisError; F6 introduces
	// Running + Idle.
	analysis AnalysisState

	// statusMessage is a one-line message shown beneath the
	// header. F4 uses it for the "Press q to quit" line; F6 uses
	// it for the spinner status ("Running security review…").
	statusMessage string

	// review is the result of the use case. nil while the use
	// case is running.
	review *reviewdomain.Review

	// errorMsg is set when analysis == AnalysisError.
	errorMsg string
}

// New creates a Model initialised with the given Review. The
// analysis state is set to AnalysisDone so the overview renders
// the final counts immediately. To start in AnalysisRunning,
// use NewRunning instead.
func New(review reviewdomain.Review) *Model {
	r := review
	return &Model{
		view:           ViewOverview,
		analysis:       AnalysisDone,
		review:         &r,
		statusMessage:  "press q to quit",
	}
}

// NewError creates a Model initialised in the error state with
// the given message. The TUI shows the message + lets the user
// quit.
func NewError(msg string) *Model {
	return &Model{
		view:          ViewOverview,
		analysis:      AnalysisError,
		errorMsg:      msg,
		statusMessage: "press q to quit",
	}
}

// Init is the bubbletea entry point. We do not issue any initial
// Cmd today; the model is fully populated by the caller before
// tea.NewProgram starts.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles incoming bubbletea messages. F4 only handles
// WindowSize + KeyMsg (q to quit). F5/F6/F7 add the rest.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	return m, nil
}

// View renders the current screen. F4 only renders the overview;
// F5/F6/F7 add the other views.
func (m *Model) View() tea.View {
	switch m.view {
	case ViewOverview:
		return tea.NewView(m.viewOverview())
	case ViewFindings:
		// F5: render the findings list.
		return tea.NewView("findings list (TODO F5)")
	case ViewDetail:
		// F5: render the finding detail.
		return tea.NewView("finding detail (TODO F5)")
	default:
		return tea.NewView("unknown view")
	}
}

// reviewOrNil returns the underlying review, or nil if it is
// not yet populated. Used by viewOverview to safely access the
// fields without nil checks scattered across the view.
func (m *Model) reviewOrNil() *reviewdomain.Review {
	return m.review
}