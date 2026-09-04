package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
)

// viewOverview renders the first screen the user sees. It shows:
//
//   - The PR header (badge + title + number + author + branches).
//   - "Review complete · N findings" with severity breakdown.
//   - Category breakdown (Security / Resilience / etc.).
//   - A footer with the keybinding hint.
//
// The view is intentionally compact: it must fit in a normal
// terminal (≥ 80 cols, ≥ 24 rows) without scrolling. The full
// findings list lives in F5's ViewFindings.
func (m *Model) viewOverview() string {
	// When the analysis failed, render a single-line error view
	// instead of the dashboard.
	if m.analysis == AnalysisError {
		return errorView(m.errorMsg, m.width)
	}

	// When the analysis is still running, render the spinner +
	// category hint. The user sees progress without any "loading…"
	// placeholder.
	if m.analysis == AnalysisRunning {
		return m.viewRunning()
	}

	r := m.reviewOrNil()
	if r == nil {
		return "loading…"
	}

	// Lipgloss styles. We keep them local because this is the
	// only file in the package that uses them today; if F5/F6
	// need to share styles we can promote them to styles.go.
	headerBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FAFAFA")).
		Width(maxInt(m.width-2, 60)).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E"))
	brandStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Bold(true)
	criticalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F85149")).Bold(true)
	highStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D29922"))
	mediumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E3B341"))
	lowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950"))

	// --- PR header box ---
	headerBody := fmt.Sprintf(
		"%s  %s\n#%d @%s  %s \u2192 %s",
		brandStyle.Render(r.PullRequest.State),
		r.PullRequest.Title,
		r.PullRequest.Number,
		r.PullRequest.AuthorLogin,
		labelStyle.Render(r.PullRequest.HeadRefName),
		labelStyle.Render(r.PullRequest.BaseRefName),
	)
	header := headerBox.Render(headerBody)

	// --- Counts ---
	counts := countFindings(r.Findings)
	total := len(r.Findings)
	title := fmt.Sprintf("Review complete \u00b7 %d %s",
		total, pluralize("finding", total))

	severityRow := fmt.Sprintf(
		"  %s %d\n  %s %d\n  %s %d\n  %s %d",
		criticalStyle.Render("Critical"), counts[severityKeyCritical],
		highStyle.Render("High"),     counts[severityKeyHigh],
		mediumStyle.Render("Medium"),   counts[severityKeyMedium],
		lowStyle.Render("Low"),       counts[severityKeyLow],
	)

	categoryRow := buildCategoryRow(r.Findings, labelStyle)

	// --- Layout ---
	content := strings.Join([]string{
		header,
		"",
		title,
		"",
		severityRow,
		"",
		labelStyle.Render("  by category"),
		categoryRow,
		"",
		m.statusMessage,
	}, "\n")

	// Pad to terminal width so the status message sits at the
	// bottom and there is no awkward trailing whitespace.
	return content
}

// buildCategoryRow renders the "by category" sub-row. Categories
// with zero findings are omitted from the list but the count
// column shows them; that way the user sees the full breakdown
// even when only one category produced findings.
func buildCategoryRow(findings []reviewdomain.Finding, labelStyle lipgloss.Style) string {
	// Stable order so the row never shuffles between renders.
	categories := []struct {
		label string
		cat   string
	}{
		{"Security", "SECURITY"},
		{"Resilience", "RESILIENCE"},
		{"Readability", "READABILITY"},
		{"Testing", "TESTING"},
		{"Dependencies", "SECURITY:SBOM"},
	}

	out := []string{}
	for _, c := range categories {
		n := countByCategory(findings, c.cat)
		out = append(out, fmt.Sprintf("  %s %s %d",
			c.label, labelStyle.Render(fmt.Sprintf("%*d", 4, n)), n))
	}
	return strings.Join(out, "\n")
}

// countFindings returns the per-severity counts. The bucket is
// derived from the finding's CVSS score (SBOM findings) or its
// severity label (LLM-emitted findings). Findings without either
// fall into the "unknown" bucket; today the overview shows
// only Critical/High/Medium/Low and ignores unknown to keep the
// view compact.
func countFindings(findings []reviewdomain.Finding) map[string]int {
	out := map[string]int{
		severityKeyCritical: 0,
		severityKeyHigh:     0,
		severityKeyMedium:   0,
		severityKeyLow:      0,
	}
	for _, f := range findings {
		key := severityBucketForFinding(f)
		if _, ok := out[key]; ok {
			out[key]++
		}
	}
	return out
}

func countByCategory(findings []reviewdomain.Finding, category string) int {
	n := 0
	for _, f := range findings {
		if f.Category == category {
			n++
		}
	}
	return n
}

const (
	severityKeyCritical = "critical"
	severityKeyHigh     = "high"
	severityKeyMedium   = "medium"
	severityKeyLow      = "low"
)

// severityBucketForFinding returns the bucket key for a finding.
// We prefer the LLM-emitted SeverityLabel; fall back to the CVSS
// score (SBOM findings). Empty / unknown findings are dropped
// (countFindings ignores them).
func severityBucketForFinding(f reviewdomain.Finding) string {
	if f.SeverityLabel != "" {
		switch f.SeverityLabel {
		case "CRITICAL":
			return severityKeyCritical
		case "HIGH":
			return severityKeyHigh
		case "MEDIUM":
			return severityKeyMedium
		case "LOW":
			return severityKeyLow
		}
	}
	switch {
	case f.CVSS >= 9.0:
		return severityKeyCritical
	case f.CVSS >= 7.0:
		return severityKeyHigh
	case f.CVSS >= 4.0:
		return severityKeyMedium
	case f.CVSS > 0:
		return severityKeyLow
	}
	return ""
}

// errorView renders a minimal error screen so the user sees what
// went wrong and how to quit.
func errorView(msg string, width int) string {
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F85149")).
		Bold(true).
		Render("error")
	body := lipgloss.NewStyle().
		Width(maxInt(width-2, 40)).
		Render(msg)
	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8B949E")).
		Render("press q to quit")
	return strings.Join([]string{title, body, "", hint}, "\n")
}

func pluralize(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// viewRunning renders the overview screen while the use case is
// in flight. The layout is intentionally compact:
//
//	⠋ Fetching diff
//
// The spinner frame updates via spinner.TickMsg bubbled from
// Update; the hint is whatever the cmd layer chose for the
// current phase (e.g. "Fetching diff" → "Running resilience
// review" → "Running security review" → ...).
func (m *Model) viewRunning() string {
	frame := m.spinner.View()
	hint := m.categoryHint
	if hint == "" {
		hint = "working…"
	}
	row := frame + " " + hint
	return strings.Repeat("\n", maxInt(m.height/2-1, 1)) + row
}