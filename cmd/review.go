package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/review/domain"
	gitAdapter "github.com/LucasNav6/code-review-cli/internal/scm/adapters/git"
	osvAdapter "github.com/LucasNav6/code-review-cli/internal/scanners/adapters/osv"
	"github.com/LucasNav6/code-review-cli/internal/logging"
	"github.com/LucasNav6/code-review-cli/internal/llm/resolver"
	"github.com/LucasNav6/code-review-cli/internal/prompts/adapters/fs"
	"github.com/LucasNav6/code-review-cli/internal/review/usecase"
	"github.com/LucasNav6/code-review-cli/internal/scm/adapters/gh"
	"github.com/LucasNav6/code-review-cli/internal/scm/adapters/storage"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

// F3 stub: runReview still lives in cmd/review.go because the
// sub-command is still registered. F7 replaces the body with
// the bubbletea TUI launch. For now we just call the use case
// and write a one-line summary to stdout so the user has
// confirmation the wiring works.

func newReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Review a GitHub Pull Request with an LLM",
		Long: `
Fetches a GitHub Pull Request and runs an LLM-powered review across
resilience, maintainability, security and testing categories.

Examples:
  code-review review --url https://github.com/owner/repo/pull/123

The LLM provider is selected via 'code-review config set provider <name>'.
There is no --provider flag; configuration is the only way to pick the
backend. All categories run by default; there is no --type flag.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          runReview,
	}
	return cmd
}

func runReview(cmd *cobra.Command, _ []string) error {
	url, err := cmd.Flags().GetString("url")
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeCommandFlag, exitCode, helpers.ErrCommandFlag)
	}

	review, err := buildReview(cmd.Context(), cmd.OutOrStdout(), os.Stderr, url)
	if err != nil {
		return mapReviewError(err)
	}

	// F3 stub: print a one-line summary. F7 swaps this for the
	// bubbletea TUI.
	fmt.Fprintf(cmd.OutOrStdout(),
		"review complete: %d findings on %s #%d\n",
		len(review.Findings),
		review.PullRequest.Title,
		review.PullRequest.Number,
	)
	return nil
}

// buildReview wires the use case from concrete adapters and
// returns the domain.Review result. Extracted so the future TUI
// (F7) can call the same wiring.
func buildReview(ctx context.Context, _, _ io.Writer, url string) (domain.Review, error) {
	scmClient := gh.New()
	store := storage.NewFileStore()
	loader := fs.New()
	sbomScanner := osvAdapter.New()
	repoFetcher := gitAdapter.New()

	uc := usecase.New(scmClient, store, resolver.New(), loader,
		sbomScanner, repoFetcher)

	return uc.Execute(ctx, usecase.ReviewPRInput{URL: url})
}

// reviewErrorMapping maps every documented fatal sentinel to the
// logging.ErrorType that surfaces it to the user. Order matters
// because the switch walks top-to-bottom; keep ErrSCMBinaryMissing
// above ErrSCMBinaryUnavailable so the "not installed" hint wins
// over the generic "unavailable".
var reviewErrorMapping = []struct {
	sentinel  error
	errorType logging.ErrorType
}{
	{scmdomain.ErrInvalidPRURL, logging.ErrorTypeCommandFlag},
	{scmdomain.ErrSCMBinaryMissing, logging.ErrorTypeGitHubCLIUnavailable},
	{scmdomain.ErrSCMBinaryInvalid, logging.ErrorTypeGitHubCLIUnavailable},
	{scmdomain.ErrSCMBinaryUnavailable, logging.ErrorTypeGitHubCLIUnavailable},
	{scmdomain.ErrDiffFetchFailed, logging.ErrorTypeDiffFetch},
	{scmdomain.ErrMetadataFetchFailed, logging.ErrorTypeMetadata},
}

// mapReviewError converts the use case error into the right
// logging.ErrorType + exit code.
func mapReviewError(err error) error {
	for _, m := range reviewErrorMapping {
		if errors.Is(err, m.sentinel) {
			return logging.LogError(os.Stderr, m.errorType, exitCode, err)
		}
	}
	return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
}