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
		updateStyle.Render("Nueva versión de code-review disponible"),
		updateStyle.Render(result.Current),
		updateStyle.Render(result.Latest),
		updateStyle.Render("Ejecutá code-review upgrade para actualizar"),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(updateAccent).
		Padding(1, 2).
		Render(inner)
}
