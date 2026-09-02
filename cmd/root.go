package cmd

import (
	"os"
	"github.com/spf13/cobra"
	"github.com/LucasNav6/code-review-cli/internal/logging"
	"github.com/LucasNav6/code-review-cli/internal/version"
	"github.com/LucasNav6/code-review-cli/ui"
)

const exitCode = 1

var (
	urlFlag     string
	versionFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "code-review",
	Short: "AI-powered review of GitHub Pull Requests",
	Long: `
code-review fetches a Pull Request and asks an LLM
to review it across four categories: resilience, maintainability, security
and testing.

Use "code-review config" to choose the LLM provider.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		switch {
			case versionFlag:
				return version.Print(cmd.OutOrStdout(), ui.MutedStyle)
			default:
				return runReviewCommand(cmd)
			}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
		os.Exit(exitCode)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&urlFlag, "url", "",
		"GitHub Pull Request URL")

	rootCmd.PersistentFlags().BoolVarP(&versionFlag, "version", "v", false,
		"Print version information and exit")

	rootCmd.AddCommand(
		newReviewCmd(),
		newUpgradeCmd(),
		newDowngradeCmd(),
		newUninstallCmd(),
		newConfigCmd(),
	)
}
