package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/internal/buildinfo"
	"github.com/LucasNav6/code-review-cli/internal/update"
)

const upgradeTimeout = 2 * time.Minute

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Actualiza code-review a la última versión disponible",

		SilenceUsage: true,

		RunE: runUpgrade,
	}
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	fmt.Println(brandStyle.Render("code-review upgrade"))
	fmt.Println(mutedStyle.Render("Versión actual: " + buildinfo.Version))
	fmt.Println()

	ctx, cancel := context.WithTimeout(cmd.Context(), upgradeTimeout)
	defer cancel()

	fmt.Println(mutedStyle.Render("Buscando la última versión..."))

	result, err := update.Upgrade(ctx)
	if err != nil {
		switch {
		case errors.Is(err, update.ErrUpToDate):
			fmt.Println(successStyle.Render("✓ Ya tenés la última versión (" + buildinfo.Version + ")"))
			return nil

		case errors.Is(err, update.ErrNotConfigured):
			fmt.Println(mutedStyle.Render(err.Error()))
			return nil

		default:
			return err
		}
	}

	fmt.Println(successStyle.Render(
		fmt.Sprintf("✓ Actualizado %s → %s", result.PreviousVersion, result.NewVersion),
	))
	fmt.Println(mutedStyle.Render(result.InstalledPath))

	return nil
}
