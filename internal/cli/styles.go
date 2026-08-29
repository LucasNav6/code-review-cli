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

	// updateAccent es la única excepción deliberada a la paleta en escala
	// de grises: reservada exclusivamente para avisos de actualización
	// (el aviso post-revisión), igual que el clásico aviso amarillo de
	// "outdated" de npm.
	updateAccent = lipgloss.Color("#F59E0B")

	updateStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(updateAccent)

	// yellowBg/yellowFg son Tailwind yellow-50 y yellow-950: el banner de
	// "Update required" en --version se pinta completo (fondo, borde y
	// texto) con este par, para que el contraste quede garantizado sin
	// depender del tema de la terminal.
	yellowBg = lipgloss.Color("#FEFCE8")
	yellowFg = lipgloss.Color("#422006")

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
