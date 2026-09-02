package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// PRHeader renders the small rounded box that summarises the pull
// request under review. The structure is:
//
//	╭──────────────────────────────────────────────────────╮
//	│ [OPEN]  Title of the pull request                    │
//	│ #5298 @author from head/branch to base/branch        │
//	╰──────────────────────────────────────────────────────╯
//
// Colour rules:
//   - borders, label literals ([, ], #, @, from, to) → foreground (white).
//   - title → foreground (white). The title reads as the heading, not data.
//   - number → muted (grey). It is data, not structure.
//   - branches (head/base) → muted + underlined. They are data and
//     benefit from the visual cue that they look like identifiers.
//   - status badge → semantic colour (green OPEN, purple MERGED,
//     red CLOSED, amber DRAFT).
type PRHeader struct {
	Number      int
	Title       string
	State       string // "OPEN", "MERGED", "CLOSED"
	IsDraft     bool
	AuthorLogin string
	HeadRef     string
	BaseRef     string
}

// Render returns the formatted box as a single string with a trailing
// newline so the next prompt or log line starts on a fresh row.
func (h PRHeader) Render() string {
	mutedStyle := MutedStyle
	mutedUnderlineStyle := MutedStyle.Underline(true)
	fgStyle := lipgloss.NewStyle().Foreground(Default.Fg)
	badgeStyle := badgeStyleFor(h.State, h.IsDraft)

	title := h.Title
	if title == "" {
		title = "(no title)"
	}

	line1 := fgStyle.Render("[") +
		badgeStyle.Render(badgeLabel(h.State, h.IsDraft)) +
		fgStyle.Render("]  ") +
		fgStyle.Render(title)

	line2 := fgStyle.Render("#") + mutedStyle.Render(itoa(h.Number)) +
		" " + fgStyle.Render("@") + mutedStyle.Render(h.AuthorLogin) +
		" " + fgStyle.Render("from") + " " + mutedUnderlineStyle.Render(h.HeadRef) +
		" " + fgStyle.Render("to") + " " + mutedUnderlineStyle.Render(h.BaseRef)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Default.Fg).
		Padding(0, 1).
		Width(TerminalWidth())

	body := line1 + "\n" + line2
	return box.Render(body) + "\n"
}

// badgeLabel returns the textual badge shown in the box header. Drafts
// always win over OPEN because they are a sub-state of OPEN.
func badgeLabel(state string, draft bool) string {
	if draft {
		return "DRAFT"
	}
	switch strings.ToUpper(state) {
	case "OPEN":
		return "OPEN"
	case "MERGED":
		return "MERGED"
	case "CLOSED":
		return "CLOSED"
	default:
		return state
	}
}

// badgeStyleFor returns the lipgloss style used to paint the status
// badge. The mapping is semantic so the colour carries meaning, not
// just decoration.
func badgeStyleFor(state string, draft bool) lipgloss.Style {
	if draft {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#D29922")) // Warning
	}
	switch strings.ToUpper(state) {
	case "OPEN":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#3FB950")) // Success
	case "MERGED":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A371F7")) // Purple (merged = completed)
	case "CLOSED":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(Danger)) // Danger
	default:
		return lipgloss.NewStyle().Bold(true).Foreground(Default.Muted)
	}
}

// itoa avoids importing strconv for a single integer-to-string conversion
// in a hot render path. h.Number is always small (PR numbers fit in 32 bits).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
