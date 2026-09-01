package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	internalupdate "github.com/LucasNav6/code-review-cli/internal/update"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "update",
		Short:         "Update code-review to the latest release",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			message := internalupdate.StatusMessage()
			_, err := fmt.Fprintln(cmd.OutOrStdout(), message)
			return err
		},
	}
}
