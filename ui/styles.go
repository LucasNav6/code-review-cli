package ui

import "charm.land/lipgloss/v2"

// Styles derived from the theme. Only the styles actually used by the
// CLI today live here — when new commands need new presentations the
// new style is added next to the command that consumes it, not the
// other way around. This keeps the package tiny and avoids dead code.
//
// Style values are package-level so callers can reuse them without
// rebuilding the lipgloss chain on every render. They are safe to
// share: lipgloss.Style is a value type in v2.
var (
	// BrandStyle paints the application name and section headers in
	// the brand colour. It is the only style that should appear in
	// the command-line help output.
	BrandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Default.Fg)

	// MutedStyle paints secondary text (descriptions, examples).
	MutedStyle = lipgloss.NewStyle().
			Foreground(Default.Muted)

	// HintStyle paints the actionable suggestion that follows an
	// error message ("Run `code-review --url ...`", etc.).
	HintStyle = lipgloss.NewStyle().
			Foreground(Default.Hint)

	// ErrorStyle paints the error message itself in the danger
	// colour. Combined with ErrorPrefixStyle it produces the
	// multi-line error rendering the CLI uses.
	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Default.Danger)

	// ErrorPrefixStyle paints the "exit status (N)" line and the
	// "╰─▶" connector. It is intentionally not bold so the error
	// body stands out.
	ErrorPrefixStyle = lipgloss.NewStyle().
				Foreground(Default.Muted)
)
