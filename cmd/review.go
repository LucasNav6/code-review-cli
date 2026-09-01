package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/claudereview"
	"github.com/LucasNav6/code-review-cli/internal/config"
	"github.com/LucasNav6/code-review-cli/internal/llm"
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

// providerFlag is the value the user passed via --provider. When empty,
// the review falls back to the persisted config and then to the default.
var providerFlag string

func newReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "review",
		Short:         "Store a GitHub Pull Request diff",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReviewCommand(cmd)
		},
	}

	// --provider overrides the persisted config for a single invocation.
	cmd.Flags().StringVar(&providerFlag, "provider", "",
		"LLM provider to use (claude, codex) — overrides the saved config")

	return cmd
}

func runReviewCommand(cmd *cobra.Command) error {
	// 1) Get the URL of the --url flag
	url, err := cmd.Flags().GetString("url")
	if err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeCommandFlag, exitCode, helpers.ErrCommandFlag)
	}

	// 2) Validate that the configured LLM provider is installed.
	// gh is also validated here (it is mandatory for the diff fetch
	// below); failures are fatal so the rest of the command does not
	// run on a broken machine.
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

	// 5) Ask the configured LLM provider to review the diff using
	// the resilience checklist. Non-blocking: failures here only emit
	// a warning so the user still gets the diff and the header they
	// would have had otherwise.
	runLLMReview(cmd)

	return nil
}

// runLLMReview loads the cached diff, builds the prompt, and pipes it
// through whatever provider is configured (or the one selected by
// --provider). All logging goes through logging.LogWarn — the
// function never returns errors to the caller because every internal
// failure is non-blocking by design.
func runLLMReview(cmd *cobra.Command) {
	provider, providerName, err := resolveProvider(cmd)
	if err != nil {
		logging.LogWarn(os.Stderr, fmt.Sprintf("LLM review skipped: %v", err))
		return
	}

	// Provider stub: codex returns ErrProviderNotImpl for both validate
	// and run. Surface that as the spinner message so the user sees the
	// provider name on screen.
	spinnerMessage := fmt.Sprintf("Reviewing diff with %s", providerName)

	var response string

	runErr := loading.Run(os.Stderr, spinnerMessage, func() error {
		diff, loadErr := internalreview.LoadStoredDiff()
		if loadErr != nil {
			return loadErr
		}

		prompt, buildErr := internalreview.BuildPrompt(resiliencePromptPath, diff)
		if buildErr != nil {
			return buildErr
		}

		if validateErr := provider.ValidateInstalled(cmd.Context()); validateErr != nil {
			return validateErr
		}

		out, runErr := provider.Run(cmd.Context(), prompt)
		if runErr != nil {
			return runErr
		}

		response = out
		return nil
	})

	if runErr != nil {
		logging.LogWarn(os.Stderr, fmt.Sprintf("LLM review skipped: %v", runErr))
		return
	}

	renderClaudeResponse(os.Stdout, response)
}

// resolveProvider picks the LLM provider for the current invocation.
// The lookup order is:
//  1. --provider flag (highest priority, overrides everything)
//  2. the value persisted in the config file
//  3. the default ("claude")
func resolveProvider(cmd *cobra.Command) (llm.Provider, string, error) {
	name := providerFlag
	if name == "" {
		store, err := config.DefaultStore()
		if err == nil {
			saved, getErr := store.GetProvider()
			if getErr == nil {
				name = saved
			}
		}
	}
	if name == "" {
		name = config.DefaultProvider
	}

	if !config.KnownProvider(name) {
		return nil, "", fmt.Errorf("%w: %q (known: %v)", helpers.ErrInvalidConfigValue, name, config.KnownProviderNames())
	}

	provider, err := llm.New(name)
	if err != nil {
		return nil, name, err
	}
	return provider, name, nil
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
		Accent:     ui.Accent,
	})

	fmt.Fprintln(w, "─── Claude review ───")

	if len(findings) == 0 {
		renderer.RenderNoFindings(w)
		return
	}

	for i, finding := range findings {
		if i > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w)
		}
		renderer.Render(w, finding)
	}
}