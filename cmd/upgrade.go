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

// upgradeConfirmFlag forces the upgrade to run without prompting.
// Default behaviour is to ask for confirmation before replacing
// the binary on disk.
var upgradeConfirmFlag bool

// upgradeTargetVersion is the explicit version requested via
// `--<version>` (e.g. `--1.0.0`). Empty means "use the default
// latest". The user specifies the version with the same name as
// the version flag on the CLI: `--1.0.0`, `--2.3.4-beta1`, etc.
//
// cobra does not natively support flags whose names look like
// version numbers (a leading dash). We capture the value in
// PreRunE by walking the raw args: any token starting with `--`
// that is NOT a known cobra flag becomes the explicit version.
//
// The default behaviour (no flag specified) is the same as
// `--latest`: pull the newest stable release.
var upgradeTargetVersion string

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade code-review to a newer release",
		Long: `Upgrade downloads a release of code-review from GitHub Releases
and replaces the current binary in place.

By default 'upgrade' pulls the latest stable release. To target
a specific version, pass it as a flag:

  code-review upgrade --latest            (default)
  code-review upgrade --1.0.0             (specific version)
  code-review upgrade --2.3.4-beta1       (prerelease)

The two modes are mutually exclusive; passing both is an error.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          runUpgrade,
	}

	cmd.Flags().BoolVarP(&upgradeConfirmFlag, "yes", "y", false,
		"Skip the confirmation prompt before replacing the binary")

	// --latest is the default; we declare it explicitly so the
	// help text shows the mode.
	cmd.Flags().Bool("latest", true,
		"Upgrade to the latest stable release (default)")

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		upgradeTargetVersion = ""
		for _, a := range args {
			// Match "--<something>" that is not a known cobra flag.
			// We accept it as an explicit version request.
			if len(a) > 2 && a[:2] == "--" {
				name := a[2:]
				// Skip known cobra keywords so the user can
				// pass them without ambiguity.
				switch name {
				case "latest", "yes", "help", "url", "version":
					continue
				}
				upgradeTargetVersion = name
			}
		}
		return nil
	}

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

	// Resolve which release to install. Latest-by-default; if the
	// user passed a specific version we look it up.
	var release *updater.Release
	if upgradeTargetVersion == "" {
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
	} else {
		if err := loading.Run(os.Stderr,
			fmt.Sprintf("Looking up release %s", upgradeTargetVersion), func() error {
				r, err := updater.FindRelease(cmd.Context(), upgradeTargetVersion)
				if err != nil {
					return err
				}
				if r == nil {
					return fmt.Errorf("release %q not found in %s",
						upgradeTargetVersion, updater.HostOf())
				}
				release = r
				return nil
			}); err != nil {
			return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
		}
	}

	// Only short-circuit "already on latest" when the user did
	// NOT request a specific version (downgrades and same-version
	// re-installs still proceed).
	if upgradeTargetVersion == "" && !release.IsNewer(buildinfo.Version) {
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