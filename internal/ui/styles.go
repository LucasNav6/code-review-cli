package ui

import "charm.land/lipgloss/v2"

// Paleta estrictamente en escala de grises (blanco, negro y grises
// intermedios), estilo minimalista shadcn — sin acentos de color. La
// jerarquía visual se logra con peso tipográfico (bold), símbolos (✓ ! ✗)
// y contraste de bordes, no con matices.
var (
	fg           = lipgloss.Color("#FAFAFA")
	bg           = lipgloss.Color("#09090B")
	muted        = lipgloss.Color("#71717A")
	dim          = lipgloss.Color("#52525B")
	border       = lipgloss.Color("#3F3F46")
	strongBorder = lipgloss.Color("#E4E4E7")

	// primary queda como alias de fg: lo usa el spinner, que antes tomaba
	// el color de marca directamente.
	primary = fg

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	brandBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(bg).
			Background(fg).
			Padding(0, 1)

	mutedStyle = lipgloss.NewStyle().
			Foreground(muted)

	dimStyle = lipgloss.NewStyle().
			Foreground(dim)

	// successStyle es texto normal: el símbolo (✓) ya transmite el estado,
	// no hace falta un color aparte.
	successStyle = lipgloss.NewStyle().
			Foreground(fg)

	// warningStyle y errorStyle usan negrita para llamar la atención sin
	// recurrir a un acento de color.
	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	infoStyle = lipgloss.NewStyle().
			Foreground(muted)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	normalPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(0, 1)

	activePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(strongBorder).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	fileStyle = lipgloss.NewStyle().
			Foreground(muted)

	categoryStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)
)
