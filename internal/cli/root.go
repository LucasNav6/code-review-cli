// Package cli define la interfaz de línea de comandos de code-review:
// parseo de flags, subcomandos (upgrade) y el punto de entrada que arranca
// la revisión interactiva.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/internal/buildinfo"
)

var (
	urlFlag  string
	orgFlag  string
	repoFlag string
	idFlag   int
)

// Execute construye el árbol de comandos y lo ejecuta. Es lo único que
// invoca cmd/code-review/main.go.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render("✗ ")+err.Error())
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code-review",
		Short: "Revisión automática de Pull Requests con Claude Code",
		Long: brandStyle.Render("code-review") + "\n" +
			mutedStyle.Render("Revisión automática de Pull Requests de GitHub con Claude Code,\n"+
				"organizada en etapas: seguridad OWASP, mantenibilidad, testing y resiliencia."),
		Example: exampleText(),
		Version: buildinfo.Version,

		SilenceErrors: true,
		SilenceUsage:  true,

		RunE: runReview,
	}

	cmd.Flags().StringVar(&urlFlag, "url", "", "URL del pull request")
	cmd.Flags().StringVar(&orgFlag, "org", "", "Organización u owner del repositorio")
	cmd.Flags().StringVar(&repoFlag, "repo", "", "Nombre del repositorio")
	cmd.Flags().IntVar(&idFlag, "id", 0, "Número del pull request")

	cmd.SetVersionTemplate(versionText())

	cmd.AddCommand(newUpgradeCmd())

	return cmd
}

func exampleText() string {
	return "  code-review --url=\"https://github.com/org/repo/pull/123\"\n" +
		"  code-review --org=\"org\" --repo=\"repo\" --id=123\n" +
		"  code-review upgrade"
}

func versionText() string {
	return fmt.Sprintf(
		"%s %s\n%s\n",
		brandStyle.Render("code-review"),
		successStyle.Render(buildinfo.Version),
		mutedStyle.Render(fmt.Sprintf("commit %s · built %s", buildinfo.Commit, buildinfo.Date)),
	)
}
