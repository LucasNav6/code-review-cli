// Package usecase owns the ReviewPRUseCase: one Execute call
// orchestrates the full review flow and returns a domain.Review
// for the bubbletea TUI (F4-F7) to render.
//
// F3 broke the old OutputSink/Notifier contract: the use case no
// longer paints. It returns a domain.Review and the cmd layer
// decides how to display it. The sbom + repoFetcher ports stay
// because the SBOM scan still produces findings (now returned
// in the Review, not painted).
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	llmdomain "github.com/LucasNav6/code-review-cli/internal/llm/domain"
	"github.com/LucasNav6/code-review-cli/internal/claudereview"
	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	scannersdomain "github.com/LucasNav6/code-review-cli/internal/scanners/domain"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

// ReviewPRInput is the parameter object Execute accepts. The
// URL is the only required field today; --type / --provider were
// removed in F1.
type ReviewPRInput struct {
	URL string
}

// ReviewPRUseCase orchestrates one full review of a pull request.
// Execute runs the SCM checks, fetches diff + metadata, builds the
// SBOM context, runs the LLM pass per category, and returns a
// domain.Review. Fatal errors abort and return (zero, err);
// non-fatal per-category failures are captured as a missing
// finding (the category simply contributes nothing to Review).
type ReviewPRUseCase struct {
	scm         SCM
	store       DiffStore
	resolver    LLMProviderResolver
	loader      PromptLoader
	scanner     SBOMScanner     // optional: nil disables SBOM
	repoFetcher RepoFetcher      // optional: nil disables SBOM
}

// New builds the use case with all its dependencies. The scanner
// and repoFetcher are OPTIONAL: passing nil for either disables
// SBOM scanning. The LLM provider resolver, loader, scm, store
// are required (a nil here is a programming error).
func New(
	scm SCM,
	store DiffStore,
	resolver LLMProviderResolver,
	loader PromptLoader,
	scanner SBOMScanner,
	repoFetcher RepoFetcher,
) *ReviewPRUseCase {
	if scm == nil || store == nil || resolver == nil || loader == nil {
		panic("usecase.New: scm, store, resolver, loader are required")
	}
	return &ReviewPRUseCase{
		scm:         scm,
		store:       store,
		resolver:    resolver,
		loader:      loader,
		scanner:     scanner,
		repoFetcher: repoFetcher,
	}
}

// Execute runs the full review flow and returns a domain.Review.
// Fatal errors abort and return (zero, err); per-category failures
// simply do not contribute findings to the Review.
//
// Fatal errors (the use case stops and returns the error):
//   - URL fails the shape check.
//   - SCM validation fails (gh not installed, etc.).
//   - Diff fetch fails.
//   - Metadata fetch fails.
//   - ResolveCategories fails (unknown --type, future).
//
// Non-fatal (category contributes no findings):
//   - Provider resolution fails.
//   - Provider validation fails.
//   - Prompt load fails.
//   - LLM run fails.
//   - Render / parse fails.
func (uc *ReviewPRUseCase) Execute(ctx context.Context, in ReviewPRInput) (reviewdomain.Review, error) {
	// 1. Validate the URL.
	prURL, err := scmdomain.NewPRURL(in.URL)
	if err != nil {
		return reviewdomain.Review{}, fmt.Errorf("validate url: %w", err)
	}

	// 2. Validate the SCM.
	if err := uc.scm.Validate(ctx); err != nil {
		return reviewdomain.Review{}, fmt.Errorf("validate scm: %w", err)
	}

	// 3. Fetch + store the diff.
	diff, err := uc.scm.FetchDiff(ctx, prURL)
	if err != nil {
		return reviewdomain.Review{}, fmt.Errorf("fetch diff: %w", err)
	}
	if err := uc.store.Save(diff); err != nil {
		return reviewdomain.Review{}, fmt.Errorf("store diff: %w", err)
	}

	// 4. Fetch metadata.
	meta, err := uc.scm.FetchMetadata(ctx, prURL)
	if err != nil {
		return reviewdomain.Review{}, fmt.Errorf("fetch metadata: %w", err)
	}

	// 5. Run every known category in canonical order. Today the
	// list is fixed (no --type flag); future F-X may let the
	// user narrow it.
	categories := reviewdomain.CanonicalOrder()

	// 6. Build the SBOM context once (only if SECURITY:SBOM is
	// in the category set). Failures are non-fatal: the SBOM
	// category contributes no findings.
	sbomText, _ := uc.buildSBOMContext(ctx, categories, prURL)

	// 7. Per-category loop. Each pass appends its findings to
	// the result. Per-category failures simply contribute zero
	// findings (the review continues).
	review := reviewdomain.Review{PullRequest: meta}
	for _, cat := range categories {
		findings, err := uc.runOne(ctx, cat, diff.Body, sbomText)
		if err != nil {
			// Non-fatal: skip the category.
			continue
		}
		review.Findings = append(review.Findings, findings...)
	}

	return review, nil
}

// buildSBOMContext scans the repo for SBOM data when any category
// in the requested set is CategorySecuritySBOM. Returns the
// JSON-encoded vulnerabilities (or "" if no SBOM is requested or
// the scanner is unavailable).
func (uc *ReviewPRUseCase) buildSBOMContext(ctx context.Context, categories []reviewdomain.Category, prURL scmdomain.PRURL) (string, error) {
	if !needsSBOM(categories) {
		return "", nil
	}
	if uc.scanner == nil || uc.repoFetcher == nil {
		return "", fmt.Errorf("scanner not configured")
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("%w: %v", scannersdomain.ErrScannerUnavailable, err)
	}

	repoPath, err := uc.repoFetcher.Clone(ctx, prURL)
	if err != nil {
		return "", fmt.Errorf("clone repo: %w", err)
	}
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

// runOne runs one category pass and returns its findings (zero
// or more) plus a non-fatal error. The caller appends the findings
// to the Review and continues on error.
//
// Provider resolution: the user picks the LLM via config
// (`code-review config set provider <name>`). Today the resolver
// returns the default when no override is passed (which is the
// only mode after F1).
func (uc *ReviewPRUseCase) runOne(ctx context.Context, cat reviewdomain.Category, diffBody []byte, sbom string) ([]reviewdomain.Finding, error) {
	provider, err := uc.resolver.Resolve("")
	if err != nil {
		return nil, fmt.Errorf("resolve provider for %s: %w", cat, err)
	}

	promptFile := reviewdomain.PromptForCategory(cat)
	templateBody, err := uc.loader.Load(promptFile, PromptContext{
		Diff: string(diffBody),
		SBOM: sbom,
	})
	if err != nil {
		return nil, fmt.Errorf("load prompt %s: %w", cat, err)
	}

	prompt := substituteDiff(templateBody, diffBody)
	prompt = substituteSBOM(prompt, sbom)

	if err := provider.ValidateInstalled(ctx); err != nil {
		return nil, fmt.Errorf("provider %s not installed: %w", provider.Name(), err)
	}

	response, err := provider.Run(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("provider %s run failed: %w", provider.Name(), err)
	}

	claudeFindings, err := claudereview.Parse(response)
	if err != nil {
		return nil, fmt.Errorf("parse %s response: %w", cat, err)
	}

	// Convert claudereview.Finding (the LLM wire format) to
	// reviewdomain.Finding (the TUI / consumer format). Today
	// the two are structurally identical; F-X may split them
	// further if we add scanner-derived fields.
	out := make([]reviewdomain.Finding, 0, len(claudeFindings))
	for _, f := range claudeFindings {
		out = append(out, findingFromClaude(f))
	}
	return out, nil
}

// findingFromClaude converts a claudereview.Finding (the parsed
// LLM response shape) into a reviewdomain.Finding (the consumer
// shape). The two structs are structurally identical today; this
// conversion is a single named hop that documents where the
// boundary lives.
//
// When the two diverge (e.g. scanner-derived fields land in
// domain.Finding only), this function is the place to bridge.
func findingFromClaude(f claudereview.Finding) reviewdomain.Finding {
	snippets := make([]reviewdomain.Snippet, len(f.Snippets))
	for i, s := range f.Snippets {
		snippets[i] = reviewdomain.Snippet{Line: s.Line, Code: s.Code}
	}
	return reviewdomain.Finding{
		Title:           f.Title,
		Context:         f.Context,
		Impact:          f.Impact,
		Suggestion:      f.Suggestion,
		Category:        string(f.Category),
		Subcategory:     f.Subcategory,
		File:            f.File,
		Line:            f.Line,
		Snippets:        snippets,
		OWASP:           f.OWASP,
		CVE:             f.CVE,
		CVSS:            f.CVSS,
		Component:       f.Component,
		ComponentVersion: f.ComponentVersion,
		FixedVersion:    f.FixedVersion,
		SeverityLabel:   f.SeverityLabel,
	}
}

// substituteDiff replaces every occurrence of {{DIFF}} in template
// with body. If the placeholder is missing, body is appended so
// the LLM still sees the diff.
func substituteDiff(template string, body []byte) string {
	const placeholder = "{{DIFF}}"
	if !strings.Contains(template, placeholder) {
		return template + "\n\n" + string(body)
	}
	return strings.ReplaceAll(template, placeholder, string(body))
}

// substituteSBOM replaces every occurrence of {{SBOM}} in template
// with body. Empty body is a no-op so the LLM sees the literal
// placeholder when no SBOM was provided.
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
// compact JSON string for {{SBOM}} substitution.
func encodeSBOM(result scannersdomain.VulnerabilityResult) string {
	if len(result.Vulnerabilities) == 0 {
		return ""
	}
	b, err := json.Marshal(result.Vulnerabilities)
	if err != nil {
		return fmt.Sprintf("%d vulnerabilities found in %s",
			len(result.Vulnerabilities), result.RepoPath)
	}
	return string(b)
}

// _ ensures the llmdomain import stays referenced for the LLM
// provider name on errors. The actual Provider type comes from
// the resolver which already depends on llmdomain.
var _ llmdomain.Provider = nil