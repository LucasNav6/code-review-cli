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

// promptDir is where the bundled prompt templates live. Each
// `<category>.md` file is loaded at run time and substituted with the
// cached diff via the {{DIFF}} placeholder.
//
// Every prompt emits the same JSON shape (see internal/claudereview)
// so a single parser can serve the four categories.
const promptDir = "internal/prompts"

const (
	promptResilience      = "resilience.md"
	promptMaintainability = "maintainability.md"
	promptSecurity        = "security.md"
	promptTesting         = "testing.md"
)

// Sentinel used by promptPathFor to mean "run every category
// sequentially". Kept as a constant so the routing logic in
// runReviewCommand stays declarative.
const reviewTypeAll = "all"

// promptPathsFor returns the ordered list of prompt filenames
// associated with the value of --type. Unknown values fall back to
// the resilience prompt so the CLI never silently does nothing.
//
// "all" expands to the four canonical categories in the order they
// appear in the prompts directory. The order is the order shown to
// the user, so keep it stable.
func promptPathsFor(t string) []string {
	switch t {
	case claudereview.CategoryReadability:
		return []string{promptMaintainability}
	case claudereview.CategorySecurity:
		return []string{promptSecurity}
	case claudereview.CategoryTesting:
		return []string{promptTesting}
	case reviewTypeAll:
		return []string{
			promptResilience,
			promptMaintainability,
			promptSecurity,
			promptTesting,
		}
	case claudereview.CategoryResilience, "":
		return []string{promptResilience}
	default:
		// Unknown type → log warning and fall back. We do not
		// return an error here because the routing decision happens
		// after the flag is read, and failing hard would be more
		// surprising than running the default review.
		fmt.Fprintf(os.Stderr, "unknown --type %q, falling back to %s\n",
			t, claudereview.CategoryResilience)
		return []string{promptResilience}
	}
}

// categoryForPrompt maps a prompt filename back to its category. It
// is used by the renderer pipeline to decide which category-specific
// extras to display. Unknown filenames fall back to resilience so the
// CLI never panics on a malformed prompt dir.
func categoryForPrompt(promptFile string) string {
	switch promptFile {
	case promptMaintainability:
		return claudereview.CategoryReadability
	case promptSecurity:
		return claudereview.CategorySecurity
	case promptTesting:
		return claudereview.CategoryTesting
	default:
		return claudereview.CategoryResilience
	}
}

// promptPath returns the absolute path of a prompt file relative to
// the current working directory. The path is passed to
// internal/review.BuildPrompt which knows how to read+substitute it.
func promptPath(name string) string {
	return promptDir + "/" + name
}

// reviewTypeFlag is the value the user passed via --type. When empty,
// the review falls back to the default (resilience).
var reviewTypeFlag string

// providerFlag is the value the user passed via --provider. When empty,
// the review falls back to the persisted config and then to the default.
var providerFlag string

func newReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Review a GitHub Pull Request with an LLM",
		Long: `Fetches a GitHub Pull Request and runs an LLM-powered review across
resilience, maintainability, security and testing categories.

Examples:
  code-review review --url https://github.com/owner/repo/pull/123
  code-review review --url <pr-url> --type security
  code-review review --url <pr-url> --provider codex`,
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

	// --type selects which review category to run. The default is
	// "all" so a plain `code-review review` invocation runs the four
	// canonical categories sequentially. Pass an explicit value to
	// restrict the run to a single category (useful for CI, for
	// iterating on a single prompt, or to keep token usage low).
	cmd.Flags().StringVar(&reviewTypeFlag, "type", reviewTypeAll,
		"Review category to run (resilience, maintainability, security, testing, all)")

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
	// the categories selected by --type. Non-blocking: failures here
	// only emit a warning so the user still gets the diff and the
	// header they would have had otherwise.
	//
	// --type all loops sequentially over the four categories; each
	// category keeps its own spinner + render so the user can see
	// progress on each pass instead of one stalled spinner.
	for _, promptFile := range promptPathsFor(reviewTypeFlag) {
		runLLMReview(cmd, promptFile)
	}

	return nil
}

// runLLMReview loads the cached diff, builds the prompt for one
// category, and pipes it through whatever provider is configured (or
// the one selected by --provider). All logging goes through
// logging.LogWarn — the function never returns errors to the caller
// because every internal failure is non-blocking by design.
//
// promptFile is the bare filename (e.g. "security.md") relative to
// the bundled prompts directory.
func runLLMReview(cmd *cobra.Command, promptFile string) {
	category := categoryForPrompt(promptFile)

	provider, providerName, err := resolveProvider(cmd)
	if err != nil {
		logging.LogWarn(os.Stderr, fmt.Sprintf("LLM review skipped: %v", err))
		return
	}

	// Provider stub: codex returns ErrProviderNotImpl for both validate
	// and run. Surface that as the spinner message so the user sees the
	// provider name on screen.
	spinnerMessage := fmt.Sprintf("Reviewing diff with %s (%s)", providerName, category)

	var response string

	runErr := loading.Run(os.Stderr, spinnerMessage, func() error {
		diff, loadErr := internalreview.LoadStoredDiff()
		if loadErr != nil {
			return loadErr
		}

		prompt, buildErr := internalreview.BuildPrompt(promptPath(promptFile), diff)
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

	renderClaudeResponse(os.Stdout, response, category)
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

// renderClaudeResponse parses claude's response, matches each finding
// against the cached diff, and prints one styled block per finding.
// A "NO_FINDINGS" or empty response renders as a single green box.
//
// category is the review category that produced response. It is used
// only as a header label ("─── Claude review (security) ───") so the
// user can tell which block corresponds to which --type pass when
// running --type all.
//
// On parse failure we print the raw response so the user still sees
// claude's answer rather than nothing.
func renderClaudeResponse(w *os.File, response string, category string) {
	findings, err := claudereview.Parse(response)
	if err != nil {
		logging.LogWarn(w, fmt.Sprintf("could not parse claude response: %v", err))
		fmt.Fprintf(w, "─── Claude review (raw, %s) ───\n", category)
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

	fmt.Fprintf(w, "─── Claude review (%s) ───\n", category)

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
