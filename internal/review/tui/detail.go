package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// viewDetail renders the focused finding as a scrollable card.
//
// The layout is intentionally flat (no scrollable code blocks
// today; we render the first snippet inline if any). F-X can
// upgrade the snippets area to its own scrollable region if the
// user reports wanting to inspect large diffs.
func (m *Model) viewDetail() string {
	r := m.reviewOrNil()
	if r == nil || len(r.Findings) == 0 {
		return "no finding selected"
	}

	// m.selectedIndex is set by the findings list when the user
	// presses Enter. The detail view reads it directly so the
	// selection state is shared between the two views (Enter /
	// Esc round-trip).
	f := r.Findings[m.selectedIndex]

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Bold(true)
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E"))
	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#58A6FF")).
		Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA"))

	var b strings.Builder

	// --- Header: category + title + location ---
	b.WriteString(headerStyle.Render(strings.ToUpper(f.Category)))
	b.WriteByte('\n')
	b.WriteString(titleStyle.Render(f.Title))
	b.WriteString("\n")
	if f.File != "" {
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		b.WriteString(mutedStyle.Render(loc))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	// --- Context ---
	if f.Context != "" {
		b.WriteString(sectionStyle.Render("Context"))
		b.WriteByte('\n')
		b.WriteString(bodyStyle.Render(f.Context))
		b.WriteString("\n\n")
	}

	// --- Impact ---
	if len(f.Impact) > 0 {
		b.WriteString(sectionStyle.Render("Impact"))
		b.WriteByte('\n')
		for _, item := range f.Impact {
			b.WriteString(mutedStyle.Render("  \u2022 " + item))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	// --- SBOM-specific blocks ---
	if f.CVE != "" {
		b.WriteString(sectionStyle.Render("CVE"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("  " + f.CVE))
		b.WriteString("\n\n")
	}
	if f.Component != "" {
		version := f.ComponentVersion
		if version == "" {
			version = "?"
		}
		b.WriteString(sectionStyle.Render("Component"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render(fmt.Sprintf("  %s @ %s", f.Component, version)))
		b.WriteString("\n\n")
	}
	if f.SeverityLabel != "" || f.CVSS > 0 {
		label := f.SeverityLabel
		if label == "" {
			label = "UNKNOWN"
		}
		score := ""
		if f.CVSS > 0 {
			score = fmt.Sprintf(" (CVSS %.1f)", f.CVSS)
		}
		b.WriteString(sectionStyle.Render("Severity"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("  " + label + score))
		b.WriteString("\n\n")
	}
	if f.FixedVersion != "" {
		b.WriteString(sectionStyle.Render("Fixed in"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("  upgrade to " + f.FixedVersion))
		b.WriteString("\n\n")
	}

	// --- Code snippet ---
	if len(f.Snippets) > 0 {
		b.WriteString(sectionStyle.Render("Code"))
		b.WriteByte('\n')
		// Render only the first snippet today (the common case is
		// one snippet per finding). F-X can iterate over all of
		// them if a scanner starts producing multiple.
		s := f.Snippets[0]
		codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#A5D6FF"))
		b.WriteString(codeStyle.Render(s.Code))
		b.WriteString("\n\n")
	}

	// --- OWASP (for SECURITY findings) ---
	if f.OWASP != "" {
		b.WriteString(sectionStyle.Render("OWASP"))
		b.WriteByte('\n')
		b.WriteString(mutedStyle.Render("  " + f.OWASP))
		b.WriteString("\n\n")
	}

	// --- Suggestion ---
	if f.Suggestion != "" {
		b.WriteString(sectionStyle.Render("Suggestion"))
		b.WriteByte('\n')
		b.WriteString(bodyStyle.Render(f.Suggestion))
		b.WriteString("\n\n")
	}

	// --- Footer: hint ---
	hint := mutedStyle.Render("[esc] back to findings list")
	b.WriteString(hint)

	return padBottom(b.String(), m.height)
}

// padBottom pads s with blank lines so it fills m.height rows.
// We add the lines AFTER the content (not before) so the content
// sticks to the top — which is what users expect when scrolling.
func padBottom(s string, height int) string {
	if height <= 0 {
		return s
	}
	rows := strings.Count(s, "\n") + 1
	if rows >= height {
		return s
	}
	pad := strings.Repeat("\n", height-rows)
	return s + pad
}