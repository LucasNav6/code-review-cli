package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/buildinfo"
	"github.com/LucasNav6/code-review-cli/internal/loading"
	"github.com/LucasNav6/code-review-cli/internal/logging"
	"github.com/LucasNav6/code-review-cli/internal/updater"
)

// downgradeTargetFlag es la versión explícita a la que se quiere
// bajar. Es obligatoria: hacer un downgrade "a la versión anterior"
// sin saber cuál es es ambiguo y peligroso (¿anterior semver? ¿anterior
// en el tiempo? ¿la última que NO sea prerelease?).
var downgradeTargetFlag string

func newDowngradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "downgrade",
		Short: "Downgrade code-review to an earlier published release",
		Long: `Downgrade downloads a specific earlier release of code-review and
replaces the current binary in place.

You MUST specify a target version with --to (for example v1.0.0).
A downgrade is refused if the target is not strictly older than the
currently running version.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          runDowngrade,
	}

	cmd.Flags().StringVar(&downgradeTargetFlag, "to", "",
		"Target version to downgrade to (e.g. v1.0.0 or 1.0.0)")
	_ = cmd.MarkFlagRequired("to")

	return cmd
}

func runDowngrade(cmd *cobra.Command, _ []string) error {
	if !buildinfo.UpdatesConfigured() {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("updates are not configured (buildinfo.Repo is empty)"))
	}

	if buildinfo.IsDev() {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("cannot downgrade a 'dev' build"))
	}

	if !updater.CanSelfUpgrade() {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("%s", updater.SelfUpgradeDisabledReason()))
	}

	target := downgradeTargetFlag
	if target == "" {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("--to is required"))
	}

	// Validamos formato semver acá mismo (antes de pegarle a GitHub)
	// para no gastar una request si el usuario tipeó cualquier verdura.
	if err := validateSemverTarget(target); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	var release *updater.Release
	if err := loading.Run(os.Stderr,
		fmt.Sprintf("Looking up release %s", target), func() error {
			r, err := updater.FindRelease(cmd.Context(), target)
			if err != nil {
				return err
			}
			if r == nil {
				return fmt.Errorf("release %q not found in %s",
					target, updater.HostOf())
			}
			release = r
			return nil
		}); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	if !release.IsOlder(buildinfo.Version) {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("refusing to downgrade: release %s is not older than current %s",
				release.TagName, buildinfo.Version))
	}

	exe, err := os.Executable()
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("could not resolve current binary: %w", err))
	}

	var result updater.InstallResult
	if err := loading.Run(os.Stderr,
		fmt.Sprintf("Downgrading to %s", release.TagName), func() error {
			r, err := updater.Downgrade(cmd.Context(), release, exe)
			if err != nil {
				return err
			}
			result = r
			return nil
		}); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"downgraded to %s at %s\n", result.Version, result.Path)
	return nil
}

// validateSemverTarget se asegura de que el target sea "vX.Y.Z" o
// "X.Y.Z" (pre-releases tipo v1.2.3-rc1 también se aceptan).
// Devuelve un error con mensaje accionable si no.
func validateSemverTarget(target string) error {
	if target == "" {
		return errors.New("--to cannot be empty")
	}

	// golang.org/x/mod/semver.IsValid es estricto; para no obligar
	// al usuario a tipear el "v", lo aceptamos y lo pasamos igual:
	// normaliseTag (en release.go) ya se encarga de prefijar.
	normalised := target
	if len(normalised) > 0 && normalised[0] != 'v' && normalised[0] != 'V' {
		normalised = "v" + normalised
	}

	// semver.IsValid quiere minúsculas en el prefijo; lo probamos
	// tal cual y, si falla, lo reintentamos en minúsculas para no
	// romper el caso "V1.2.3" (raro pero pasa).
	check := normalised
	if check[0] == 'V' {
		check = "v" + check[1:]
	}

	// Importamos el paquete semver indirectamente a través de una
	// llamada barata a FindRelease sería un desperdicio, así que
	// validamos con una regex laxa + la pasamos por el filter real
	// cuando se busca la release.
	if !looksLikeSemver(check) {
		return fmt.Errorf("--to %q is not a valid semver version (expected vX.Y.Z, e.g. v1.2.3)", target)
	}
	return nil
}

// looksLikeSemver es una validación barata para evitar round-trips
// a GitHub con cualquier verdura. NO reemplaza al check real
// (semver.IsValid); sólo es un guard temprano.
func looksLikeSemver(v string) bool {
	if len(v) < 5 { // "v0.0.0"
		return false
	}
	if v[0] != 'v' {
		return false
	}
	dots := 0
	for i := 1; i < len(v); i++ {
		c := v[i]
		if c == '.' {
			dots++
			continue
		}
		if (c < '0' || c > '9') && c != '-' && (c < 'a' || c > 'z') {
			return false
		}
	}
	return dots == 2
}
