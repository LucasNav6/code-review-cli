package cli

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/LucasNav6/code-review-cli/internal/buildinfo"
	"github.com/LucasNav6/code-review-cli/internal/update"
)

// printUpdateNoticeIfAny se ejecuta al final de una revisión y, si hay una
// versión más nueva publicada, imprime una card estilo npm avisándolo.
// El chequeo usa una caché en disco, así que no agrega latencia salvo la
// primera vez por día.
func printUpdateNoticeIfAny() {
	if !buildinfo.UpdatesConfigured() {
		return
	}

	result, err := update.Check()
	if err != nil || result == nil || !result.HasUpdate {
		return
	}

	fmt.Println()
	fmt.Println(renderUpdateCard(result))
}

func renderUpdateCard(result *update.CheckResult) string {
	inner := fmt.Sprintf(
		"%s\n\n%s  →  %s\n\n%s",
		warningStyle.Bold(true).Render("Nueva versión de code-review disponible"),
		mutedStyle.Render(result.Current),
		successStyle.Bold(true).Render(result.Latest),
		mutedStyle.Render("Ejecutá ")+brandStyle.Render("code-review upgrade")+mutedStyle.Render(" para actualizar"),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(warningColor).
		Padding(1, 2).
		Render(inner)
}
