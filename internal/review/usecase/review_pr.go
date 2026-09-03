package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
	scannersdomain "github.com/LucasNav6/code-review-cli/internal/scanners/domain"
)

// ReviewPRInput is the parameter object Execute accepts. Keeping it
// as a struct (rather than positional args) means new fields can be
// added without breaking call sites.
type ReviewPRInput struct {
	// URL is the raw PR URL as the user typed it. The use case
	// validates and normalises it through scmdomain.NewPRURL before
	// any SCM call is made, so downstream code can rely on the
	// shape.
	URL string

	// ReviewType is the value the user passed via --type (or the
	// empty string for the default). The use case routes this
	// through reviewdomain.ResolveCategories.
	ReviewType reviewdomain.ReviewType

	// ProviderOverride is the value the user passed via --provider.
	// An empty string means "use the persisted config / default";
	// the resolver handles the rest.
	ProviderOverride string
}

// ReviewPRUseCase orchestrates one full review of a pull request.
// It owns the order of operations (validate SCM → fetch diff →
// fetch metadata → render header → per-category LLM review) and
// the failure semantics (some failures abort, others only emit a
// warning and continue).
//
// Execute is the only public entry point. All other types and
// helpers are unexported because the use case is meant to be used
// as a single black box from the cmd layer.
type ReviewPRUseCase struct {
	scm         SCM
	store       DiffStore
	resolver    LLMProviderResolver
	loader      PromptLoader
	sink        OutputSink
	notifier    Notifier
	scanner     SBOMScanner     // optional: nil disables SBOM
	repoFetcher RepoFetcher      // optional: nil disables SBOM
}

// New builds the use case with all its dependencies. The caller
// (composition root) is responsible for providing concrete (or
// fake, in tests) implementations of every port. There is no
// default — passing nil is a programming error.
//
// scanner and repoFetcher are OPTIONAL: passing nil for either
// disables SBOM scanning. This keeps backward compatibility with
// callers (mainly tests) that did not wire the scanner during the
// H-S1 / H-S1.5 era.
func New(
	scm SCM,
	store DiffStore,
	resolver LLMProviderResolver,
	loader PromptLoader,
	sink OutputSink,
	notifier Notifier,
	scanner SBOMScanner,
	repoFetcher RepoFetcher,
) *ReviewPRUseCase {
	if scm == nil || store == nil || resolver == nil ||
		loader == nil || sink == nil || notifier == nil {
		panic("usecase.New: all dependencies are required")
	}
	return &ReviewPRUseCase{
		scm:         scm,
		store:       store,
		resolver:    resolver,
		loader:      loader,
		sink:        sink,
		notifier:    notifier,
		scanner:     scanner,
		repoFetcher: repoFetcher,
	}
}

// Execute runs the full review flow. Returns the FIRST fatal error
// encountered; non-fatal per-category failures are surfaced through
// the Notifier and do not abort the flow.
//
// Fatal errors (the use case stops and returns the error):
//
//   - URL fails the shape check.
//   - SCM validation fails (gh not installed, etc.).
//   - Diff fetch fails (network, auth, PR not found).
//   - Metadata fetch fails (same family).
//   - ResolveCategories fails (unknown --type).
//
// Non-fatal errors (the use case logs via notifier.Warn and
// continues with the next category):
//
//   - Per-category provider resolution fails (unknown provider).
//   - Per-category provider validation fails.
//   - Per-category prompt load fails.
//   - Per-category LLM run fails.
//   - Per-category render fails.
//
// The split is intentional: the first batch is about "we cannot
// review at all"; the second is about "this one pass did not work".
func (uc *ReviewPRUseCase) Execute(ctx context.Context, in ReviewPRInput) error {
	// 1. Validate the URL shape so we fail fast on typos like
	// "github.com/owner/repo" (repo URL, no /pull/<n>).
	prURL, err := scmdomain.NewPRURL(in.URL)
	if err != nil {
		return fmt.Errorf("validate url: %w", err)
	}

	// 2. Validate the SCM binary is installed. Fatal: without it
	// nothing else can run.
	if err := uc.scm.Validate(ctx); err != nil {
		return fmt.Errorf("validate scm: %w", err)
	}

	// 3. Fetch the diff and persist it. Fatal: subsequent
	// categories need the diff to build their prompts.
	diff, err := uc.scm.FetchDiff(ctx, prURL)
	if err != nil {
		return fmt.Errorf("fetch diff: %w", err)
	}
	if err := uc.store.Save(diff); err != nil {
		return fmt.Errorf("store diff: %w", err)
	}

	// 4. Fetch metadata + render the header box. Fatal: a missing
	// header leaves the user with no summary of what they are
	// reviewing.
	meta, err := uc.scm.FetchMetadata(ctx, prURL)
	if err != nil {
		return fmt.Errorf("fetch metadata: %w", err)
	}
	uc.sink.RenderHeader(meta)

	// 5. Resolve the categories from --type. An unknown value
	// here is treated as fatal so the user does not run a silent
	// zero-category review.
	categories, err := reviewdomain.ResolveCategories(in.ReviewType)
	if err != nil {
		return fmt.Errorf("resolve categories: %w", err)
	}

	// 6. Per-category loop. Each pass is independent and
	// non-blocking; failures are reported via notifier.Warn and
	// the loop moves on.
	//
	// The SBOM context is computed once (before the loop) and
	// passed to every category that might want it. Today only
	// CategorySecuritySBOM consumes it; the others ignore it.
	sbom, sbomErr := uc.buildSBOMContext(ctx, categories, prURL)
	if sbomErr != nil {
		// SBOM failures are non-fatal: log a warning and let the
		// categories run without SBOM data. The LLM for SBOM
		// review will see an empty {{SBOM}} and emit no
		// findings (or fall back gracefully).
		uc.notifier.Warn(fmt.Sprintf("SBOM scan skipped: %v", sbomErr))
	}

	for _, cat := range categories {
		uc.runOne(ctx, cat, diff.Body, in.ProviderOverride, sbom)
	}

	return nil
}

// buildSBOMContext scans the repo for SBOM data when any category
// in the requested set is CategorySecuritySBOM. If no SBOM
// category was requested, it returns an empty string without
// invoking the scanner (no overhead for non-SBOM reviews).
//
// On any error (no repo path, scanner unavailable, network) the
// caller receives (empty, err) so it can decide whether to abort
// the whole review or just emit a warning. Today we treat every
// SBOM error as non-fatal.
func (uc *ReviewPRUseCase) buildSBOMContext(ctx context.Context, categories []reviewdomain.Category, prURL scmdomain.PRURL) (string, error) {
	if !needsSBOM(categories) {
		return "", nil
	}
	if uc.scanner == nil || uc.repoFetcher == nil {
		return "", fmt.Errorf("scanner not configured")
	}

	// Clone the repo to a tmpdir. We honour the caller's ctx for
	// the clone network round-trip; on failure we return an
	// error so the caller can decide whether to surface a warn.
	repoPath, err := uc.repoFetcher.Clone(ctx, prURL)
	if err != nil {
		return "", fmt.Errorf("clone repo: %w", err)
	}

	// Scan the working tree. The scanner walks the dir looking
	// for lockfiles (go.mod, package-lock.json, etc.) and
	// queries the OSV database for CVEs.
	result, err := uc.scanner.Scan(ctx, repoPath)
	if err != nil {
		return "", fmt.Errorf("scan sbom: %w", err)
	}

	return encodeSBOM(result), nil
}

// needsSBOM reports whether any of the requested categories
// consumes the SBOM context. Today only CategorySecuritySBOM.
func needsSBOM(categories []reviewdomain.Category) bool {
	for _, c := range categories {
		if c == reviewdomain.CategorySecuritySBOM {
			return true
		}
	}
	return false
}

// runOne runs a single category pass end-to-end:
//  1. resolve the provider for the current override,
//  2. load the prompt template,
//  3. substitute {{DIFF}} and {{SBOM}} as needed,
//  4. validate the provider is installed,
//  5. run the LLM,
//  6. hand the raw response to the sink for rendering.
//
// Any failure is reported via notifier.Warn and the function returns
// so the next category can proceed. The caller never sees a
// category-level error.
//
// We pass diffBody (not the diff object) so the function does not
// need to know about the Diff type and so tests can call it with
// a literal []byte.
func (uc *ReviewPRUseCase) runOne(ctx context.Context, cat reviewdomain.Category, diffBody []byte, providerOverride, sbom string) {
	// 6.1. Resolve the provider. Use the override if the user
	// passed --provider; otherwise let the resolver fall back to
	// the persisted config / default.
	provider, err := uc.resolver.Resolve(providerOverride)
	if err != nil {
		uc.notifier.Warn(fmt.Sprintf("category %s skipped: %v", cat, err))
		return
	}

	// 6.2. Load the prompt template for this category. The
	// SBOM placeholder is empty for non-SBOM categories; the
	// loader still substitutes it (no-op when empty).
	promptFile := reviewdomain.PromptForCategory(cat)
	templateBody, err := uc.loader.Load(promptFile, PromptContext{
		Diff: string(diffBody),
		SBOM: sbom,
	})
	if err != nil {
		uc.notifier.Warn(fmt.Sprintf("category %s skipped: load prompt: %v", cat, err))
		return
	}

	// 6.3. Substitute the {{DIFF}} placeholder. We do this in
	// the use case (not the loader) so the loader is a dumb
	// read-from-disk and the substitution policy is owned by the
	// caller. The policy here matches what the existing
	// internal/review.BuildPrompt does: single substitution,
	// append if the placeholder is missing.
	prompt := substituteDiff(templateBody, diffBody)
	prompt = substituteSBOM(prompt, sbom)

	// 6.4. Validate the provider is installed AND authenticated.
	// Non-fatal so a single broken provider does not block the
	// other categories.
	if err := provider.ValidateInstalled(ctx); err != nil {
		uc.notifier.Warn(fmt.Sprintf(
			"category %s skipped: provider %s not installed: %v",
			cat, provider.Name(), err))
		return
	}

	// 6.5. Run the LLM. Non-fatal so a single broken run does
	// not block the other categories.
	response, err := provider.Run(ctx, prompt)
	if err != nil {
		uc.notifier.Warn(fmt.Sprintf(
			"category %s skipped: provider %s run failed: %v",
			cat, provider.Name(), err))
		return
	}

	// 6.6. Hand the response to the sink. Sink errors are
	// non-fatal — the use case does not care about formatting
	// failures.
	if err := uc.sink.RenderReviewBlock(cat, response); err != nil {
		uc.notifier.Warn(fmt.Sprintf("category %s render failed: %v", cat, err))
	}
}

// substituteDiff replaces every occurrence of {{DIFF}} in template
// with body. If the placeholder is missing, body is appended (this
// matches the existing internal/review.BuildPrompt behaviour so
// the migration keeps the same prompt substitution semantics).
//
// The function is a small helper kept private to the package so
// the substitution policy is owned by the use case, not the
// loader.
func substituteDiff(template string, body []byte) string {
	const placeholder = "{{DIFF}}"
	if !strings.Contains(template, placeholder) {
		return template + "\n\n" + string(body)
	}
	return strings.ReplaceAll(template, placeholder, string(body))
}

// substituteSBOM replaces every occurrence of {{SBOM}} in template
// with body. If the placeholder is missing, body is appended so
// the LLM still sees the SBOM data even when the prompt template
// forgot to reference it (defensive default for future prompts).
//
// An empty body is a no-op: the placeholder stays literal so the
// LLM sees "no SBOM data" rather than an empty injection.
func substituteSBOM(template, body string) string {
	if body == "" {
		return template
	}
	const placeholder = "{{SBOM}}"
	if !strings.Contains(template, placeholder) {
		return template + "\n\n" + body
	}
	return strings.ReplaceAll(template, placeholder, body)
}

// encodeSBOM serialises a scanners.VulnerabilityResult into a
// compact JSON string suitable for {{SBOM}} substitution. The
// shape is intentionally flat — every field is included because
// the LLM uses every one of them to assess actionability.
//
// Returns "" if the result has no vulnerabilities (the LLM gets
// an empty {{SBOM}} placeholder and emits NO_FINDINGS).
func encodeSBOM(result scannersdomain.VulnerabilityResult) string {
	if len(result.Vulnerabilities) == 0 {
		return ""
	}
	// Compact JSON: smaller prompts = lower token cost. We do
	// NOT pretty-print because the LLM does not care about
	// whitespace.
	b, err := json.Marshal(result.Vulnerabilities)
	if err != nil {
		// Fall back to a human-readable representation. The LLM
		// can parse either, but JSON is preferred when
	// available.
		return fmt.Sprintf("%d vulnerabilities found in %s",
			len(result.Vulnerabilities), result.RepoPath)
	}
	return string(b)
}