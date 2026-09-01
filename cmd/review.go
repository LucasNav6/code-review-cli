package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/claude"
	"github.com/LucasNav6/code-review-cli/internal/claudereview"
	"github.com/LucasNav6/code-review-cli/internal/loading"
	"github.com/LucasNav6/code-review-cli/internal/logging"
	"github.com/LucasNav6/code-review-cli/internal/pr"
	internalreview "github.com/LucasNav6/code-review-cli/internal/review"
	"github.com/LucasNav6/code-review-cli/ui"
)

// resiliencePromptPath is the bundled prompt template the CLI ships
// with for the resilience review. The placeholder {{DIFF}} is replaced
// at run time with the cached diff bytes.
const resiliencePromptPath = "internal/prompts/resilience.md"

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

	// 2) Validate that gh is installed. gh is mandatory for every step
	// below, so a failure here is fatal.
	if err := loading.Run(os.Stderr, "Validating GitHub CLI installation", func() error {
		return internalreview.ValidateGitHubCLI(cmd.Context())
	}); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeGitHubCLIUnavailable, exitCode, err)
	}

	// 3) Fetch and store the diff. Fatal on failure: the rest of the
	// command has nothing to review without it.
	if err := loading.Run(os.Stderr, "Fetching pull request diff with gh", func() error {
		return internalreview.StoreDiff(cmd.Context(), internalreview.NewFileStore(), url)
	}); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeDiffFetch, exitCode, err)
	}

	// 4) Fetch PR metadata and render the header box. Also fatal: a
	// missing header leaves the user with no summary of what they are
	// reviewing.
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

	// 5) Ask claude to review the diff using the resilience checklist.
	// Non-blocking: failures here only emit a warning so the user still
	// gets the diff and the header they would have had otherwise.
	runClaudeReview(cmd)

	return nil
}

// runClaudeReview loads the cached diff, builds the prompt with the
// resilience template, pipes it to `claude -p`, parses the response,
// and renders each Finding as a styled comment-style block.
//
// All logging goes through logging.LogWarn — the function never
// returns errors to the caller because every internal failure is
// non-blocking by design.
func runClaudeReview(cmd *cobra.Command) {
	var response string

	err := loading.Run(os.Stderr, "Reviewing diff with Claude", func() error {
		diff, loadErr := internalreview.LoadStoredDiff()
		if loadErr != nil {
			return loadErr
		}

		prompt, buildErr := internalreview.BuildPrompt(resiliencePromptPath, diff)
		if buildErr != nil {
			return buildErr
		}

		// Re-check claude at this point too: a user who installed or
		// authenticated claude after the first validation should still
		// get a review.
		if validateErr := internalreview.ValidateClaude(cmd.Context()); validateErr != nil {
			return validateErr
		}

		out, runErr := claude.Run(cmd.Context(), prompt)
		if runErr != nil {
			return runErr
		}

		response = out
		return nil
	})

	if err != nil {
		logging.LogWarn(os.Stderr, fmt.Sprintf("claude review skipped: %v", err))
		return
	}

	renderClaudeResponse(os.Stdout, response)
}

// renderClaudeResponse parses claude's markdown, matches each finding
// against the cached diff, and prints one styled block per finding.
// A "NO_FINDINGS" or empty response renders as a single green box.
//
// On parse failure we print the raw response so the user still sees
// claude's answer rather than nothing.
func renderClaudeResponse(w *os.File, response string) {
	findings, err := claudereview.Parse(response)
	if err != nil {
		logging.LogWarn(w, fmt.Sprintf("could not parse claude response: %v", err))
		fmt.Fprintln(w, "─── Claude review (raw) ───")
		fmt.Fprintln(w, response)
		return
	}

	renderer := claudereview.NewRenderer(claudereview.Colours{
		Foreground: ui.Fg,
		Muted:      ui.Muted,
		Added:      ui.Success,
		Removed:    ui.Danger,
		Category:   ui.Brand,
	})

	fmt.Fprintln(w, "─── Claude review ───")

	if len(findings) == 0 {
		renderer.RenderNoFindings(w)
		return
	}

	hunks := claudereview.ParseHunks(cachedDiffOrEmpty())

	for i, finding := range findings {
		if i > 0 {
			fmt.Fprintln(w)
		}
		var hunk *claudereview.Hunk
		if h, ok := claudereview.HunkForLine(hunks, finding.Archivo, finding.Linea); ok {
			hunk = &h
		}
		renderer.Render(w, finding, hunk)
	}
}

// cachedDiffOrEmpty returns the diff bytes from disk if available, or
// an empty string if the file is missing. The renderer is robust to
// an empty diff so a missing cache file does not abort the review.
func cachedDiffOrEmpty() string {
	data, err := os.ReadFile(internalreview.DiffPath())
	if err != nil {
		return ""
	}
	return string(data)
}