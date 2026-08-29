package cli

import (
	"fmt"
	"runtime"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/internal/buildinfo"
	"github.com/LucasNav6/code-review-cli/internal/update"
)

var versionStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("245"))

func init() {
	// Se registra como template para que el chequeo remoto ocurra
	// únicamente cuando Cobra necesita renderizar --version.
	cobra.AddTemplateFunc("versionInfo", renderVersionInfo)
}

func versionTemplateText() string {
	return "{{versionInfo}}"
}

// renderVersionInfo muestra la versión local y, si existe una versión
// remota diferente, agrega un aviso de actualización.
func renderVersionInfo() string {
	version := normalizeVersion(buildinfo.Version)
	goVersion := strings.TrimPrefix(runtime.Version(), "go")

	var out strings.Builder

	// La información base se muestra muted para que el aviso de actualización
	// tenga mayor jerarquía visual cuando exista.
	out.WriteString(
		versionStyle.Render(
			fmt.Sprintf("code-review %s (Go %s)", version, goVersion),
		),
	)

	// Algunos builds locales pueden no tener configurado el origen remoto
	// necesario para consultar actualizaciones.
	if !buildinfo.UpdatesConfigured() {
		out.WriteString("\n")
		return out.String()
	}

	result, err := update.Check()

	// El chequeo es informativo: --version debe seguir funcionando
	// aunque no haya conexión o falle el servicio remoto.
	if err != nil || result == nil || !result.HasUpdate {
		out.WriteString("\n")
		return out.String()
	}

	current := normalizeVersion(result.Current)
	latest := normalizeVersion(result.Latest)

	updateMessage := fmt.Sprintf(
		"> A new version of code-review is available: %s → %s\n"+
			"> Run `code-review upgrade` to update",
		current,
		latest,
	)

	out.WriteString("\n\n")
	out.WriteString(updateStyle.Render(updateMessage))
	out.WriteString("\n")

	return out.String()
}

// normalizeVersion mantiene una salida consistente independientemente
// de si la versión llega como "v0.2.0" o "0.2.0".
func normalizeVersion(version string) string {
	if version == "" {
		return "unknown"
	}

	return strings.TrimPrefix(version, "v")
}
