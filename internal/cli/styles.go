package cli

import "charm.land/lipgloss/v2"

// Estilos livianos para los mensajes que la CLI imprime fuera de la TUI
// (banners, errores, el aviso de actualización, --version, etc).
var (
	primaryColor = lipgloss.Color("#7C5CFC")
	mutedColor   = lipgloss.Color("#71717A")
	successColor = lipgloss.Color("#22C55E")
	warningColor = lipgloss.Color("#F59E0B")
	dangerColor  = lipgloss.Color("#EF4444")

	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	brandBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 1)

	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(dangerColor)
)
