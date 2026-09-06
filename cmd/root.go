package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

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
		case urlFlag != "":
			return runReview(cmd, nil)
		default:
			return cmd.Help()
		}
	},
}

func Execute() {
	// A signal-aware, cancellable context so ctrl+c/SIGTERM (delivered
	// outside the TUI's raw-mode key handling — e.g. `kill`, or ctrl+c
	// after the terminal has left raw mode) actually tears down any
	// in-flight subprocess (git clone, gh, claude) instead of leaving it
	// orphaned. See runReview in cmd/review.go for the TUI-exit path,
	// which layers its own cancellation on top of this context.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
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
