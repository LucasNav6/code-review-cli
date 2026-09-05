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
	"fmt"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

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
// F5 adds the findings list + detail view; F6 adds the spinner
// shown while the use case is running; F7 wires the TUI into cmd.
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

	// categoryHint is shown next to the spinner while the use
	// case is running (e.g. "Fetching diff" / "Running security
	// review"). Empty when analysis != AnalysisRunning.
	categoryHint string

	// review is the result of the use case. nil while the use
	// case is running.
	review *reviewdomain.Review

	// errorMsg is set when analysis == AnalysisError.
	errorMsg string

	// list is the bubbles/list.Model for the findings screen.
	// Constructed once in New; updated only when the underlying
	// review changes (which today is exactly once at startup).
	list list.Model

	// selectedIndex is the index into Review.Findings that the
	// detail view should show. Updated when the user presses
	// Enter on the list view.
	selectedIndex int

	// spinner animates the "running" overview. Nil when the use
	// case is not running (AnalysisDone / AnalysisError).
	spinner spinner.Model
}

// newList returns a fresh bubbles/list.Model sized to 80x20 with
// no items. The list is mandatory: Update calls m.list.SetSize
// when a WindowSizeMsg arrives, and SetSize on a zero-value list
// nil-derefs inside bubbles/list.updatePagination. Every Model
// constructor must call this helper.
//
// width/height defaults (80x20) are overwritten by the first
// WindowSizeMsg; they exist so the list is usable before that
// arrives.
func newList() list.Model {
	l := list.New(nil, newFindingDelegate(80), 80, 20)
	l.Title = "Findings"
	l.SetShowHelp(false) // we paint our own hint at the bottom
	return l
}

// New creates a Model initialised with the given Review. The
// analysis state is set to AnalysisDone so the overview renders
// the final counts immediately. To start in AnalysisRunning,
// use NewRunning instead.
func New(review reviewdomain.Review) *Model {
	r := review
	l := newList()
	// Populate the list now with the review's findings. We
	// rebuild it on ReviewReadyMsg in Update if the review
	// changes, so today's setup-time population is sufficient.
	l.SetItems(findingsAsItems(review.Findings))
	m := &Model{
		view:          ViewOverview,
		analysis:      AnalysisDone,
		review:        &r,
		list:          l,
		selectedIndex: 0,
		statusMessage: "press q to quit",
	}
	return m
}

// NewRunning creates a Model in the running state. The overview
// view paints a spinner + the supplied categoryHint (e.g.
// "Fetching diff" / "Running security review" / etc.) so the
// user sees progress during the long phases of the use case.
//
// The list is initialised empty; items are filled in when
// ReviewReadyMsg arrives (see Update).
func NewRunning(categoryHint string) *Model {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	return &Model{
		view:          ViewOverview,
		analysis:      AnalysisRunning,
		spinner:       s,
		list:          newList(),
		categoryHint:  categoryHint,
		statusMessage: categoryHint,
	}
}

// NewError creates a Model initialised in the error state with
// the given message. The TUI shows the message + lets the user
// quit.
//
// The list is still initialised because Update calls SetSize on
// it for any WindowSizeMsg, even when the user is looking at
// the error view.
func NewError(msg string) *Model {
	return &Model{
		view:          ViewOverview,
		analysis:      AnalysisError,
		errorMsg:      msg,
		list:          newList(),
		statusMessage: "press q to quit",
	}
}

// Init is the bubbletea entry point. We do not issue any initial
// Cmd today; the model is fully populated by the caller before
// tea.NewProgram starts.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles incoming bubbletea messages.
//
// Keybindings:
//
//	Overview view:
//
//	    q / ctrl+c  quit
//	    enter       switch to findings list (no-op while running)
//
//	Findings list:
//
//	    q / ctrl+c   quit
//	    esc          back to overview
//	    enter         open selected finding (detail view)
//	    up/k          move selection up
//	    down/j        move selection down
//
//	Detail view:
//
//	    q / ctrl+c   quit
//	    esc          back to findings list
//
// Custom messages:
//
//	ReviewReadyMsg  flips the model from AnalysisRunning to
//	                AnalysisDone (or AnalysisError). Carries the
//	                Review + Err from the use case.
//	spinner.TickMsg  forwarded to the spinner so it animates.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil

	case ReviewReadyMsg:
		// The use case finished. Flip state, populate the review
		// (or the error), rebuild the list from the new findings,
		// and stop the spinner (just clear it; we no longer render
		// it once analysis != Running).
		if msg.Err != nil {
			m.analysis = AnalysisError
			m.errorMsg = msg.Err.Error()
			return m, nil
		}
		r := msg.Review
		m.review = &r
		m.analysis = AnalysisDone
		m.statusMessage = "press q to quit"
		// Rebuild the list with the final findings.
		items := findingsAsItems(r.Findings)
		m.list = list.New(items, newFindingDelegate(m.width), m.width, m.height-4)
		m.list.Title = "Findings"
		m.selectedIndex = 0
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "enter":
			switch m.view {
			case ViewOverview:
				if m.analysis == AnalysisDone && len(m.reviewOrNil().Findings) > 0 {
					m.view = ViewFindings
				}
			case ViewFindings:
				m.selectedIndex = m.list.Index()
				m.view = ViewDetail
			}

		case "esc":
			switch m.view {
			case ViewFindings:
				m.view = ViewOverview
			case ViewDetail:
				m.view = ViewFindings
			}
		}

		// Delegate arrow keys / hjkl to the list when it is active.
		if m.view == ViewFindings {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// Forward any other messages (notably spinner.TickMsg) to
	// the spinner when it is active. This is what makes the
	// running screen animate.
	if m.analysis == AnalysisRunning && m.isSpinnerActive() {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// isSpinnerActive reports whether the model has a live spinner
// (the field is non-zero after NewRunning). We cannot compare
// the spinner.Model value directly because its fields are
// unexported; we use the analysis state as the indicator
// instead, which is the same condition.
func (m *Model) isSpinnerActive() bool {
	return m.analysis == AnalysisRunning
}

// View renders the current screen.
func (m *Model) View() tea.View {
	switch m.view {
	case ViewOverview:
		return tea.NewView(m.viewOverview())
	case ViewFindings:
		return tea.NewView(m.viewFindings())
	case ViewDetail:
		return tea.NewView(m.viewDetail())
	default:
		return tea.NewView(fmt.Sprintf("unknown view: %d", int(m.view)))
	}
}

// viewFindings renders the findings list with a one-line footer
// hint. The list is the bubbles/list.Model rendered directly;
// its internal scrolling + selection state is opaque to us.
func (m *Model) viewFindings() string {
	r := m.reviewOrNil()
	if r == nil || len(r.Findings) == 0 {
		return "no findings"
	}
	footer := "[esc] back   [enter] open"
	return m.list.View() + "\n" + footer
}

// reviewOrNil returns the underlying review, or nil if it is
// not yet populated. Used by viewOverview to safely access the
// fields without nil checks scattered across the view.
func (m *Model) reviewOrNil() *reviewdomain.Review {
	return m.review
}