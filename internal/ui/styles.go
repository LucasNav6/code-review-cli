package ui

import "charm.land/lipgloss/v2"

// Paleta de marca. Se mantiene deliberadamente chica y consistente para
// que la interfaz se sienta sólida y "empresarial" en vez de un TUI de
// hobby con muchos colores sueltos.
var (
	primary = lipgloss.Color("#7C5CFC")
	text    = lipgloss.Color("#FAFAFA")
	muted   = lipgloss.Color("#71717A")
	dim     = lipgloss.Color("#52525B")
	border  = lipgloss.Color("#3F3F46")
	success = lipgloss.Color("#22C55E")
	warning = lipgloss.Color("#F59E0B")
	danger  = lipgloss.Color("#EF4444")
	info    = lipgloss.Color("#38BDF8")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text)

	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primary)

	brandBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primary).
			Padding(0, 1)

	mutedStyle = lipgloss.NewStyle().
			Foreground(muted)

	dimStyle = lipgloss.NewStyle().
			Foreground(dim)

	successStyle = lipgloss.NewStyle().
			Foreground(success)

	warningStyle = lipgloss.NewStyle().
			Foreground(warning)

	errorStyle = lipgloss.NewStyle().
			Foreground(danger)

	infoStyle = lipgloss.NewStyle().
			Foreground(info)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primary)

	normalPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(0, 1)

	activePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primary)

	fileStyle = lipgloss.NewStyle().
			Foreground(info)

	categoryStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(warning)

	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text)
)
