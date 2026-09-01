package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/loading"
	"github.com/LucasNav6/code-review-cli/internal/logging"
	"github.com/LucasNav6/code-review-cli/internal/pr"
	internalreview "github.com/LucasNav6/code-review-cli/internal/review"
	"github.com/LucasNav6/code-review-cli/ui"
)

func newReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "review",
		Short:         "Store a GitHub Pull Request diff",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReviewCommand(cmd)
		},
	}
}

func runReviewCommand(cmd *cobra.Command) error {
	// 1) Get the URL of the --url flag
	url, err := cmd.Flags().GetString("url")
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeCommandFlag, exitCode, helpers.ErrCommandFlag)
	}

	// 2) Validate if the user has installed "gh" cli
	if err := loading.Run(os.Stderr, "Validating GitHub CLI installation", func() error {
		return internalreview.ValidateGitHubCLI(cmd.Context())
	}); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeGitHubCLIUnavailable, exitCode, err)
	}

	// 3) Get the diff of the pull request
	if err := loading.Run(os.Stderr, "Fetching pull request diff with gh", func() error {
		return internalreview.StoreDiff(cmd.Context(), internalreview.NewFileStore(), url)
	}); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeDiffFetch, exitCode, err)
	}

	// 4) Fetch PR metadata and render the header box. We only do this
	// once the diff is safely stored so a metadata failure does not
	// leave the user with no payload to review.
	meta, err := pr.Fetch(cmd.Context(), url)
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeMetadata, exitCode, err)
	}

	header := ui.PRHeader{
		Number:      meta.Number,
		Title:       meta.Title,
		State:       meta.State,
		IsDraft:     meta.IsDraft,
		AuthorLogin: meta.AuthorLogin,
		HeadRef:     meta.HeadRefName,
		BaseRef:     meta.BaseRefName,
	}
	os.Stdout.WriteString(header.Render())

	return nil
}
