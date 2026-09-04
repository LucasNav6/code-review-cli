package tui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
)

// findingItem implements bubbles/list.Item for one Finding.
// It carries the source finding + its index in the original
// Review.Findings slice so the detail view can refer back to it.
type findingItem struct {
	finding reviewdomain.Finding
	index   int
}

// FilterValue is required by list.Item. The list filters by typing;
// we match against title + file (case-insensitive) so the user can
// narrow by either.
//
// bubbles/list calls FilterValue on every keystroke during a
// search, so the implementation must be cheap. String comparisons
// are fine for the data sizes we expect (<10k findings per review).
func (i findingItem) FilterValue() string {
	return i.finding.Title + " " + i.finding.File
}

// findingDelegate paints each row of the findings list. We render
// a compact two-line block: title + file:line on the first line,
// severity tag on the second line. The exact pixel layout is
// settled by lipgloss styles so it stays consistent across views.
type findingDelegate struct {
	width int
}

// New findingDelegate returns a delegate that paints rows to fit
// the given terminal width. The width is passed in so the title
// truncation is stable when the terminal resizes.
func newFindingDelegate(width int) findingDelegate {
	return findingDelegate{width: width}
}

// Render paints one row. The format is:
//
//	┃ <severity>  <title truncated to width-20>
//	┃ <file>:<line>
//
// The leading ┃ is the bubble's selection marker (added by the
// list component itself; we do not draw it).
func (d findingDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	fi, ok := item.(findingItem)
	if !ok {
		// Defensive: bubbles/list should only carry our items.
		return
	}

	severity := severityTag(fi.finding)
	title := truncate(fi.finding.Title, d.width-20)
	location := fmt.Sprintf("%s:%d", fi.finding.File, fi.finding.Line)
	if fi.finding.Line == 0 {
		location = fi.finding.File
	}

	// Severity colour per row: red for critical, orange for high,
	// yellow for medium, green for low. Matches the overview
	// colours so the user sees the same palette in both views.
	sevStyle := severityStyle(fi.finding)
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E"))

	// Highlight the selected row with a brighter background.
	if index == m.Index() {
		sevStyle = sevStyle.Bold(true)
		titleStyle = titleStyle.Bold(true).Foreground(lipgloss.Color("#FAFAFA"))
	}

	fmt.Fprintf(w, "%s %s\n", sevStyle.Render(severity), titleStyle.Render(title))
	fmt.Fprintf(w, "  %s\n", mutedStyle.Render(location))
}

// Height is required by ItemDelegate. Two lines per row keeps the
// list compact while still showing the file:line.
func (d findingDelegate) Height() int { return 2 }

// Spacing is required by ItemDelegate. We return 0 so consecutive
// rows sit directly on top of each other (no extra blank line
// between them); the box border on the list itself handles visual
// separation.
func (d findingDelegate) Spacing() int { return 0 }

// Update is required by ItemDelegate. The list delegates per-item
// updates here. We do not need any per-item state so this is a
// no-op.
func (d findingDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

// severityTag returns the short uppercase label that goes in the
// first column of each row. We mirror the overview's severity
// mapping (CVSS -> bucket -> label).
func severityTag(f reviewdomain.Finding) string {
	bucket := severityBucketForFinding(f)
	switch bucket {
	case severityKeyCritical:
		return "CRIT"
	case severityKeyHigh:
		return "HIGH"
	case severityKeyMedium:
		return "MED "
	case severityKeyLow:
		return "LOW "
	}
	return "    "
}

// severityStyle returns a lipgloss style for a finding's severity.
// Same colours as the overview so the user sees a consistent
// palette in both views.
func severityStyle(f reviewdomain.Finding) lipgloss.Style {
	bucket := severityBucketForFinding(f)
	switch bucket {
	case severityKeyCritical:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F85149")).Bold(true)
	case severityKeyHigh:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#D29922"))
	case severityKeyMedium:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#E3B341"))
	case severityKeyLow:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950"))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E"))
}

// truncate shortens s to max characters, appending "…" when the
// input was longer. ASCII-only (good enough for English finding
// titles; multi-byte titles render with an extra character at
// the end but the layout stays consistent).
func truncate(s string, max int) string {
	if max <= 1 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// findingsAsItems converts the flat Findings slice into the
// bubbles/list Item type. The index is preserved so the detail
// view can refer back to Review.Findings[index].
func findingsAsItems(findings []reviewdomain.Finding) []list.Item {
	out := make([]list.Item, len(findings))
	for i, f := range findings {
		out[i] = findingItem{finding: f, index: i}
	}
	return out
}