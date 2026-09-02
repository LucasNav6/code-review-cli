package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/buildinfo"
	"github.com/LucasNav6/code-review-cli/internal/loading"
	"github.com/LucasNav6/code-review-cli/internal/logging"
	"github.com/LucasNav6/code-review-cli/internal/updater"
)

// upgradeConfirmFlag fuerza a upgrade a ejecutarse sin pedir
// confirmación. Por default el comando pregunta antes de pisar el
// binario en disco.
var upgradeConfirmFlag bool

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "upgrade",
		Aliases: []string{"update"},
		Short:   "Upgrade code-review to the latest stable release",
		Long: `Upgrade downloads the latest stable release of code-review from
GitHub Releases and replaces the current binary in place.

Equivalent to "update" (kept as an alias for backwards compatibility).`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          runUpgrade,
	}

	cmd.Flags().BoolVarP(&upgradeConfirmFlag, "yes", "y", false,
		"Skip the confirmation prompt before replacing the binary")

	return cmd
}

func runUpgrade(cmd *cobra.Command, _ []string) error {
	if !buildinfo.UpdatesConfigured() {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("updates are not configured (buildinfo.Repo is empty)"))
	}

	if buildinfo.IsDev() {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("cannot upgrade a 'dev' build; install a release first: %s/releases",
				updater.HostOf()))
	}

	if !updater.CanSelfUpgrade() {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("%s", updater.SelfUpgradeDisabledReason()))
	}

	// Buscamos la latest ANTES de pedir confirmación para poder
	// mostrarle al usuario qué versión se va a instalar.
	var release *updater.Release
	if err := loading.Run(os.Stderr, "Looking up the latest stable release", func() error {
		r, err := updater.LatestStable(cmd.Context())
		if err != nil {
			return err
		}
		release = r
		return nil
	}); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	if !release.IsNewer(buildinfo.Version) {
		fmt.Fprintf(cmd.OutOrStdout(),
			"already on the latest version (%s)\n", release.TagName)
		return nil
	}

	if !upgradeConfirmFlag {
		fmt.Fprintf(cmd.OutOrStdout(),
			"About to upgrade from %s to %s. Continue? [y/N] ",
			buildinfo.Version, release.TagName)
		var answer string
		if _, err := fmt.Scanln(&answer); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "aborted")
			return nil
		}
		if answer != "y" && answer != "Y" {
			fmt.Fprintln(cmd.OutOrStdout(), "aborted")
			return nil
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("could not resolve current binary: %w", err))
	}

	var result updater.InstallResult
	if err := loading.Run(os.Stderr,
		fmt.Sprintf("Upgrading to %s", release.TagName), func() error {
			r, err := updater.Upgrade(cmd.Context(), release, exe)
			if err != nil {
				return err
			}
			result = r
			return nil
		}); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"upgraded to %s at %s\n", result.Version, result.Path)
	return nil
}
