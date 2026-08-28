package cli

import "charm.land/lipgloss/v2"

// Estilos livianos para los mensajes que la CLI imprime fuera de la TUI
// (banners, errores, el aviso de actualización, --version, etc). Misma
// paleta en escala de grises que la TUI: sin acentos de color, jerarquía
// por peso tipográfico y contraste.
var (
	fg     = lipgloss.Color("#FAFAFA")
	bg     = lipgloss.Color("#09090B")
	muted  = lipgloss.Color("#71717A")
	border = lipgloss.Color("#3F3F46")

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

	successStyle = lipgloss.NewStyle().
			Foreground(fg)

	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)
)
