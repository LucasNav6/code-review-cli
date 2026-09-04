package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/helpers"
	gitAdapter "github.com/LucasNav6/code-review-cli/internal/scm/adapters/git"
	osvAdapter "github.com/LucasNav6/code-review-cli/internal/scanners/adapters/osv"
	"github.com/LucasNav6/code-review-cli/internal/logging"
	"github.com/LucasNav6/code-review-cli/internal/llm/resolver"
	"github.com/LucasNav6/code-review-cli/internal/prompts/adapters/fs"
	"github.com/LucasNav6/code-review-cli/internal/review/render"
	"github.com/LucasNav6/code-review-cli/internal/review/usecase"
	"github.com/LucasNav6/code-review-cli/internal/scm/adapters/gh"
	"github.com/LucasNav6/code-review-cli/internal/scm/adapters/storage"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

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

// runReview is the cobra RunE entry point. Its only job is to
// compose the use case from concrete adapters and execute it. All
// orchestration lives in the use case; this function does not
// branch on success/failure paths beyond mapping the returned
// error to the right logging ErrorType + exit code.
//
// Why no top-level spinner around the whole call? The earlier
// H6 implementation wrapped Execute in loading.Run, which wrote
// the spinner message to stderr with '\r' carriage returns while
// the sink wrote the header box to stdout. When the user
// redirected 2>&1 (or any TTY merge) the two streams interleaved:
// the spinner line stayed on screen overlapping the header. The
// use case already prints the header box + per-category blocks
// in sequence, which is progress enough for the user. If a
// future usecase change wants per-step progress, the right shape
// is a Spinner port the use case drives via the Notifier — not
// a cmd-level loading.Run wrapper.
func runReview(cmd *cobra.Command, _ []string) error {
	url, err := cmd.Flags().GetString("url")
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeCommandFlag, exitCode, helpers.ErrCommandFlag)
	}

	// Composition root: wire every concrete adapter the use case
	// needs. Today: gh CLI for the SCM, the production resolver
	// for the LLM, a filesystem loader for prompts, the CLI sink
	// + notifier for output, the OSV-backed SBOM scanner, and the
	// git-clone-backed RepoFetcher. Any future adapter swaps
	// happen here and nowhere else.
	scmClient := gh.New()
	store := storage.NewFileStore()
	loader := fs.New()
	sink := render.NewCLISink(os.Stdout, os.Stderr)
	notifier := render.NewCLINotifier(os.Stderr)
	sbomScanner := osvAdapter.New()
	repoFetcher := gitAdapter.New()

	uc := usecase.New(scmClient, store, resolver.New(), loader, sink, notifier,
		sbomScanner, repoFetcher)

	if err := uc.Execute(cmd.Context(), usecase.ReviewPRInput{
		URL: url,
	}); err != nil {
		return mapReviewError(err)
	}
	return nil
}

// reviewErrorMapping maps every documented fatal sentinel to the
// logging.ErrorType that surfaces it to the user. Order matters
// because the switch walks top-to-bottom; keep ErrSCMBinaryMissing
// above ErrSCMBinaryUnavailable so the "not installed" hint wins
// over the generic "unavailable".
//
// Adding a new fatal sentinel to the use case means adding one
// entry here.
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

// mapReviewError converts the error returned by ReviewPRUseCase
// into the right logging.ErrorType + exit code. The use case wraps
// context ("fetch metadata: %w") so we match on the underlying
// sentinel via errors.Is.
//
// Without this mapping the user would see a generic "exit 1" with
// no hint about what failed. We match on every documented fatal
// sentinel so the error line tells the user which step blew up.
func mapReviewError(err error) error {
	for _, m := range reviewErrorMapping {
		if errors.Is(err, m.sentinel) {
			return logging.LogError(os.Stderr, m.errorType, exitCode, err)
		}
	}
	// Fallback: log as unknown. The use case's wrapped error
	// already carries the actionable message.
	return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
}