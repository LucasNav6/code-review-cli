package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/config"
	"github.com/LucasNav6/code-review-cli/internal/logging"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "config",
		Short:         "View and edit code-review preferences",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigEdit(cmd)
		},
	}

	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigListCmd())
	cmd.AddCommand(newConfigUnsetCmd())

	return cmd
}

// newConfigSetCmd: code-review config set <key> <value>
func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "set <key> <value>",
		Short:         "Set a configuration value",
		Args:          cobra.ExactArgs(2),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSet(cmd, args[0], args[1])
		},
	}
}

// newConfigGetCmd: code-review config get <key>
func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "get <key>",
		Short:         "Print a single configuration value",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigGet(cmd, args[0])
		},
	}
}

// newConfigListCmd: code-review config list
func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "list",
		Short:         "Print every configuration value",
		Aliases:       []string{"ls"},
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigList(cmd)
		},
	}
}

// newConfigUnsetCmd: code-review config unset <key>
func newConfigUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "unset <key>",
		Short:         "Remove a configuration value (revert to default)",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigUnset(cmd, args[0])
		},
	}
}

// runConfigEdit is the default `code-review config` invocation. It
// opens the JSON file in $EDITOR (falling back to the OS default
// editor and then to vi) so the user can edit values by hand.
func runConfigEdit(cmd *cobra.Command) error {
	store, err := config.DefaultStore()
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	// Ensure the file exists so the editor opens something useful on
	// first run.
	c, err := store.Load()
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}
	if err := store.Save(c); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	editor := resolveEditor()
	bin, args := editor[0], editor[1:]

	cobraCmd := exec.Command(bin, append(args, store.Path())...)
	cobraCmd.Stdin = os.Stdin
	cobraCmd.Stdout = os.Stdout
	cobraCmd.Stderr = os.Stderr
	if err := cobraCmd.Run(); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, fmt.Errorf("editor exited with error: %w", err))
	}
	return nil
}

// resolveEditor picks the editor to use for `code-review config`. It
// honours $VISUAL, then $EDITOR, then falls back to sensible defaults.
func resolveEditor() []string {
	if v := os.Getenv("VISUAL"); v != "" {
		return []string{v}
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return []string{e}
	}
	if _, err := exec.LookPath("code"); err == nil {
		return []string{"code", "-w"}
	}
	if _, err := exec.LookPath("nano"); err == nil {
		return []string{"nano"}
	}
	return []string{"vi"}
}

func runConfigSet(cmd *cobra.Command, key, value string) error {
	store, err := config.DefaultStore()
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	switch key {
	case "provider":
		if !config.KnownProvider(value) {
			return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
				fmt.Errorf("%w: %q (known: %v)", helpers.ErrInvalidConfigValue, value, config.KnownProviderNames()))
		}
		if err := store.SetProvider(value); err != nil {
			return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "provider = %s\n", value)
		return nil
	}

	return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
		fmt.Errorf("%w: %q (known keys: provider)", helpers.ErrInvalidConfigValue, key))
}

func runConfigGet(cmd *cobra.Command, key string) error {
	store, err := config.DefaultStore()
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	switch key {
	case "provider":
		value, err := store.GetProvider()
		if err != nil {
			return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), value)
		return nil
	}

	return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
		fmt.Errorf("%w: %q (known keys: provider)", helpers.ErrInvalidConfigValue, key))
}

func runConfigList(cmd *cobra.Command) error {
	store, err := config.DefaultStore()
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	provider, err := store.GetProvider()
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "config file: %s\n", store.Path())
	fmt.Fprintf(out, "provider    = %s\n", provider)
	return nil
}

func runConfigUnset(cmd *cobra.Command, key string) error {
	store, err := config.DefaultStore()
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}

	switch key {
	case "provider":
		if err := store.UnsetProvider(); err != nil {
			return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "provider unset (default: %s)\n", config.DefaultProvider)
		return nil
	}

	return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode,
		fmt.Errorf("%w: %q (known keys: provider)", helpers.ErrInvalidConfigValue, key))
}
