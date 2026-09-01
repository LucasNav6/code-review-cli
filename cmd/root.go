// Package cmd is the bare cobra entry point of code-review.
//
// This package owns the Cobra command tree only. Runtime logic lives
// under internal/ so cmd/ stays limited to root.go, review.go and
// update.go.
package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/buildinfo"
	"github.com/LucasNav6/code-review-cli/internal/logging"
	"github.com/LucasNav6/code-review-cli/ui"
)

// exitCode is the conventional "something went wrong" status. We keep
// it as a package-level constant so callers and tests can reference it
// without re-declaring the magic number.
const exitCode = 1

// urlFlag is declared as package-level because root and review share
// it (root uses it via RunE; review reads it from cmd.Flags()).
var (
	urlFlag     string
	versionFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "code-review",
	Short: "Store a GitHub Pull Request diff",
	Long: `code-review stores the plain-text diff for a GitHub Pull Request
using the GitHub CLI.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// --version / -v short-circuits the review.
		if versionFlag {
			printVersion(cmd)
			return nil
		}

		return runReviewCommand(cmd)
	},
	Example: `  code-review --url https://github.com/org/repo/pull/123
  code-review update
  code-review --version`,
}

// Execute is the single entry point invoked from main.go. It builds the
// root command, registers subcommands, runs Cobra and translates any
// error into the process exit status with the project's logger format.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
		os.Exit(exitCode)
	}
}

// printVersion writes the binary identity line in the project's muted
// style. Single source of truth — review/update contexts that ever
// need to print the version should import this helper rather than
// rebuilding the format string.
func printVersion(cmd *cobra.Command) {
	version := strings.TrimPrefix(buildinfo.Version, "v")
	if version == "" {
		version = "unknown"
	}
	goVersion := strings.TrimPrefix(runtime.Version(), "go")
	if goVersion == "" {
		goVersion = "unknown"
	}
	commit := buildinfo.Commit
	if commit == "" {
		commit = "unknown"
	}
	date := buildinfo.Date
	if date == "" {
		date = "unknown"
	}

	line := fmt.Sprintf("code-review CLI %s (commit %s, built %s, Go %s)",
		version, commit, date, goVersion)

	cmd.Println(ui.MutedStyle.Render(line))
}

func init() {
	// --url is a persistent flag so it's available both at the root
	// (the default behaviour) and on the `review` subcommand.
	rootCmd.PersistentFlags().StringVar(&urlFlag, "url", "",
		"GitHub Pull Request URL")

	// --version / -v prints the build info in muted style and exits
	// without running the review.
	rootCmd.PersistentFlags().BoolVarP(&versionFlag, "version", "v", false,
		"Print version information and exit")

	rootCmd.AddCommand(newReviewCmd(), newUpdateCmd(), newConfigCmd())
}
