package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/internal/buildinfo"
)

var urlFlag string

// Execute builds the command tree and runs the CLI.
// It is the only entry point called from cmd/code-review/main.go.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code-review",
		Short: "AI-powered GitHub Pull Request review",
		Long: "AI-powered GitHub Pull Request review with Claude Code.\n" +
			"Analyzes security, dependencies, maintainability, testing, and resilience.",
		Example: exampleText(),
		Version: buildinfo.Version,

		// Errors and usage are rendered by our own UI instead of Cobra defaults.
		SilenceErrors: true,
		SilenceUsage:  true,

		RunE: runReview,
	}

	cmd.Flags().StringVar(
		&urlFlag,
		"url",
		"",
		"GitHub Pull Request URL",
	)

	cmd.SetVersionTemplate(versionTemplateText())
	cmd.SetFlagErrorFunc(flagErrorFunc)

	// Custom help keeps the CLI visually consistent with version and errors.
	cmd.SetHelpFunc(renderHelp)

	cmd.AddCommand(newUpgradeCmd())

	return cmd
}

func exampleText() string {
	return "  code-review --url https://github.com/org/repo/pull/123\n" +
		"  code-review upgrade"
}
