package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/internal/logging"
	"github.com/LucasNav6/code-review-cli/internal/updater"
)

// uninstallYesFlag saltea la confirmación.
var uninstallYesFlag bool

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the code-review binary from disk",
		Long: `Uninstall removes the code-review binary that is currently running.

It only works for binaries installed in places the CLI is allowed to
write to (the install script's default ~/.local/bin or $GOBIN). If the
binary was installed via Homebrew, or in a system location that cannot
be determined, the command prints the manual steps instead of deleting
anything on its own.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          runUninstall,
	}

	cmd.Flags().BoolVarP(&uninstallYesFlag, "yes", "y", false,
		"Skip the confirmation prompt")

	return cmd
}

func runUninstall(cmd *cobra.Command, _ []string) error {
	method, path, err := updater.DetectInstallMethod()
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	// Mensaje de contexto antes de pedir confirmación, para que el
	// usuario sepa qué va a pasar.
	fmt.Fprintf(cmd.OutOrStdout(),
		"Detected install method: %s (%s)\n", method, path)

	switch method {
	case updater.InstallMethodLocal, updater.InstallMethodGoBin:
		// OK, podemos borrar.

	case updater.InstallMethodHomebrew:
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("installed via Homebrew; run `brew uninstall code-review` instead"))

	default:
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
			fmt.Errorf("cannot determine how %s was installed; remove it manually", path))
	}

	if !uninstallYesFlag {
		fmt.Fprintf(cmd.OutOrStdout(),
			"This will DELETE %s from disk. Continue? [y/N] ", path)
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

	result, err := updater.Uninstall()
	if err != nil {
		// Si el error es ErrCannotUninstall, ya tiene el mensaje
		// formateado para el usuario; lo dejamos pasar tal cual.
		if errors.Is(err, updater.ErrCannotUninstall) {
			return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
				formatUninstallHint(err))
		}
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"uninstalled code-review (%s) from %s\n", result.Method, result.Path)
	return nil
}

// formatUninstallHint limpia el mensaje de error para que sea
// accionable en vez de parecer un stacktrace.
func formatUninstallHint(err error) error {
	msg := err.Error()
	// Saamos el prefijo "<errors.ErrCannotUninstall>: " que queda feo.
	msg = strings.TrimPrefix(msg, updater.ErrCannotUninstall.Error()+": ")
	return fmt.Errorf("%s", msg)
}
