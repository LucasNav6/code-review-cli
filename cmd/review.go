package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/review/domain"
	"github.com/LucasNav6/code-review-cli/internal/review/tui"
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

// newReviewCmd builds the `code-review review` sub-command.
// Kept for backward compatibility (existing scripts + docs
// mention 'code-review review'); the same logic is also reachable
// via 'code-review --url <pr>' (the root Cmd picks --url and
// dispatches to runReview when no sub-command is given).
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

// runReview launches the bubbletea TUI. The flow:
//
//  1. Build the use case from concrete adapters (composition
//     root).
//  2. Create a TUI Model in the running state. The TUI paints
//     a spinner + "Fetching diff" while the use case runs.
//  3. Spawn a goroutine that runs the use case and sends a
//     ReviewReadyMsg to the program when it returns.
//  4. Run tea.NewProgram(...).Run() which blocks until the user
//     quits (q/ctrl+c) or the goroutine sends ReviewReadyMsg + the
//     user quits.
//
// Failure modes:
//   - URL flag missing: error (the cobra cmd's Args validate this).
//   - Use case fails before launching the TUI: log the error +
//     return exit code 1 (no TUI).
//   - Use case fails inside the TUI goroutine: the goroutine
//     sends ReviewReadyMsg with Err set; the TUI shows the error
//     view; the user can quit. No non-zero exit code because
//     the TUI itself exited cleanly (user pressed q).
func runReview(cmd *cobra.Command, _ []string) error {
	url, err := cmd.Flags().GetString("url")
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeCommandFlag, exitCode, helpers.ErrCommandFlag)
	}

	// Build the use case from concrete adapters (composition
	// root). We do this BEFORE launching the TUI so any
	// configuration error (missing binary, bad adapter, etc.)
	// surfaces as a clean exit code rather than inside the TUI.
	uc, err := buildUseCase()
	if err != nil {
		return mapReviewError(err)
	}

	// Program context. We use the cobra-supplied ctx so
	// signal-driven cancellation (ctrl+c on the terminal) tears
	// down both the TUI and the use-case goroutine.
	ctx := cmd.Context()

	// Create the TUI in running state. The categoryHint tells
	// the user "what is happening" during the first phase
	// (diff + metadata fetch). F-X can layer per-category
	// hints (sending ReviewReadyMsg with category-level
	// progress) but the static hint is good enough for the
	// first commit.
	model := tui.NewRunning("Fetching diff")

	// Pre-resolve the use case synchronously so we fail fast on
	// bad URLs etc. The use case is fast for these checks
	// (no network); the long phases happen after the TUI is up.
	//
	// We run a synchronous pre-check by calling Execute and
	// inspecting the result. If the result is an error from the
	// URL/validate/diff-fetch phase, fail before launching the
	// TUI. If the result is a real Review, we use it as the
	// initial Review (no spinner needed).
	//
	// For F7 we always launch the TUI in the running state — the
	// pre-check is skipped. The user sees the spinner + hint +
	// "press q to quit" while the use case runs. Errors are
	// surfaced in the TUI itself (AnalysisError view).
	p := tea.NewProgram(model)
	defer p.Quit()

	// Goroutine: run the use case + send ReviewReadyMsg.
	go func() {
		review, err := uc.Execute(ctx, usecase.ReviewPRInput{URL: url})
		p.Send(tui.ReviewReadyMsg{Review: review, Err: err})
	}()

	// p.Run() blocks until the program exits (user quits). The
	// TUI goroutine may send ReviewReadyMsg before we get here;
	// that's fine — the program handles it before exiting.
	if _, err := p.Run(); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeUnknown, exitCode, err)
	}
	return nil
}

// buildUseCase wires the use case from concrete adapters (the
// composition root). Extracted so the TUI launch (runReview) and
// any future headless mode (CI, IDE plugin) share the same wiring.
//
// Errors here mean the environment is broken (missing binary, bad
// adapter config) and we cannot even start a review — so we
// return the error before launching the TUI.
func buildUseCase() (*usecase.ReviewPRUseCase, error) {
	scmClient := gh.New()
	store := storage.NewFileStore()
	loader := fs.New()
	sbomScanner := osvAdapter.New()
	repoFetcher := gitAdapter.New()

	return usecase.New(scmClient, store, resolver.New(), loader,
		sbomScanner, repoFetcher), nil
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

// Ensure the build references unused imports are caught early;
// these are aliases that document the F7 transition.
var (
	_ domain.Review
	_ io.Writer
	_ = fmt.Sprintf
)