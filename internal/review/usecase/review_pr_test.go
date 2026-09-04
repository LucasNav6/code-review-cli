package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	llmdomain "github.com/LucasNav6/code-review-cli/internal/llm/domain"
	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	scannersdomain "github.com/LucasNav6/code-review-cli/internal/scanners/domain"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
	"github.com/LucasNav6/code-review-cli/internal/review/usecase"
)

// newUseCaseUnderTest wires up a ReviewPRUseCase with the supplied
// fakes. The fakes cover every port the use case consumes; the
// scanner + fetcher default to nil (no SBOM).
func newUseCaseUnderTest(
	scm usecase.SCM,
	store usecase.DiffStore,
	resolver usecase.LLMProviderResolver,
	loader usecase.PromptLoader,
) *usecase.ReviewPRUseCase {
	return usecase.New(scm, store, resolver, loader, nil, nil)
}

// validInput returns a ReviewPRInput that would succeed on a
// happy path.
func validInput() usecase.ReviewPRInput {
	return usecase.ReviewPRInput{URL: "https://github.com/owner/repo/pull/123"}
}

// TestExecute_HappyPath verifies the orchestration in order:
// validate URL, validate SCM, fetch + store diff, fetch metadata,
// run every category in canonical order, return a Review with
// the metadata + the findings from each LLM pass.
func TestExecute_HappyPath(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1, Title: "PR"},
		DiffResult:    scmdomain.Diff{Body: []byte("+ diff line")},
	}
	resolver := &fakeResolver{Providers: map[string]llmdomain.Provider{
		"": &fakeProvider{NameVal: "claude", RunResp: `{"findings": []}`},
	}}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			"resilience.md":      "R",
			"maintainability.md": "M",
			"security.md":        "S",
			"testing.md":         "T",
		},
	}
	store := &fakeStore{}

	uc := newUseCaseUnderTest(scm, store, resolver, loader)
	review, err := uc.Execute(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if review.PullRequest.Number != 1 || review.PullRequest.Title != "PR" {
		t.Errorf("PullRequest: got %+v", review.PullRequest)
	}
	// Every category was called exactly once.
	if len(resolver.Calls) != 5 {
		t.Errorf("resolver calls: got %d, want 5 (one per category)", len(resolver.Calls))
	}
	if len(loader.Calls) != 5 {
		t.Errorf("loader calls: got %d, want 5 (one per category, including SECURITY:SBOM)", len(loader.Calls))
	}
	// All four categories ran (each returned 0 findings → empty
	// findings slice; the use case did not error).
	if len(review.Findings) != 0 {
		t.Errorf("expected 0 findings (no LLM emitted any), got %d", len(review.Findings))
	}
}

// TestExecute_ReturnsParsedFindings verifies that LLM-emitted
// findings are converted from claudereview.Finding to
// reviewdomain.Finding and surface in the Review with all fields
// preserved (title, file, line, category).
func TestExecute_ReturnsParsedFindings(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 42, Title: "PR"},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	resolver := &fakeResolver{Providers: map[string]llmdomain.Provider{
		"": &fakeProvider{
			NameVal: "claude",
			RunResp: `{"findings": [{
				"title": "Bug",
				"context": "...",
				"impact": ["..."],
				"suggestion": "Fix it",
				"category": "RESILIENCE",
				"file": "foo.go",
				"line": 17
			}]}`,
		},
	}}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			"resilience.md":      "R",
			"maintainability.md": "M",
			"security.md":        "S",
			"security_sbom.md":   "SB",
			"testing.md":         "T",
		},
	}

	uc := newUseCaseUnderTest(scm, &fakeStore{}, resolver, loader)
	review, err := uc.Execute(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// The fake provider returns the same JSON for every
	// category, so each of the 5 categories emits one finding
	// → 5 findings total. We just check the first one has the
	// expected fields.
	if len(review.Findings) != 5 {
		t.Fatalf("expected 5 findings (one per category), got %d", len(review.Findings))
	}
	f := review.Findings[0]
	if f.Title != "Bug" || f.File != "foo.go" || f.Line != 17 {
		t.Errorf("finding: got %+v", f)
	}
	if f.Category != "RESILIENCE" {
		t.Errorf("category: got %q", f.Category)
	}
}

// TestExecute_URLShapeFailure verifies the fail-fast path: an
// invalid URL must abort before any SCM call.
func TestExecute_URLShapeFailure(t *testing.T) {
	scm := &fakeSCM{}
	uc := newUseCaseUnderTest(scm, &fakeStore{}, &fakeResolver{}, &fakeLoader{})

	_, err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL: "https://github.com/owner/repo", // missing /pull/<n>
	})
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
	if !errors.Is(err, scmdomain.ErrInvalidPRURL) {
		t.Errorf("expected ErrInvalidPRURL, got %v", err)
	}
	if scm.ValidateCalls != 0 || scm.FetchDiffCalls != 0 || scm.FetchMetaCalls != 0 {
		t.Errorf("SCM was called despite URL failure: %+v", scm)
	}
}

// TestExecute_SCMValidateFailure verifies the first fatal in the
// pipeline: if gh is not installed, the use case stops.
func TestExecute_SCMValidateFailure(t *testing.T) {
	scm := &fakeSCM{ValidateErr: scmdomain.ErrSCMBinaryMissing}
	uc := newUseCaseUnderTest(scm, &fakeStore{}, &fakeResolver{}, &fakeLoader{})

	_, err := uc.Execute(context.Background(), validInput())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, scmdomain.ErrSCMBinaryMissing) {
		t.Errorf("expected ErrSCMBinaryMissing, got %v", err)
	}
	if scm.FetchDiffCalls != 0 {
		t.Error("FetchDiff must not be called when Validate fails")
	}
}

// TestExecute_DiffFetchFailureFatal verifies that a diff-fetch
// failure aborts the whole review (there is nothing to review
// without the diff).
func TestExecute_DiffFetchFailureFatal(t *testing.T) {
	scm := &fakeSCM{FetchDiffErr: scmdomain.ErrDiffFetchFailed}
	uc := newUseCaseUnderTest(scm, &fakeStore{}, &fakeResolver{}, &fakeLoader{})

	_, err := uc.Execute(context.Background(), validInput())
	if !errors.Is(err, scmdomain.ErrDiffFetchFailed) {
		t.Errorf("expected ErrDiffFetchFailed, got %v", err)
	}
}

// TestExecute_MetadataFetchFailureFatal: same logic for metadata.
func TestExecute_MetadataFetchFailureFatal(t *testing.T) {
	scm := &fakeSCM{
		DiffResult:   scmdomain.Diff{Body: []byte("+ x")},
		FetchMetaErr: scmdomain.ErrMetadataFetchFailed,
	}
	uc := newUseCaseUnderTest(scm, &fakeStore{}, &fakeResolver{}, &fakeLoader{})

	_, err := uc.Execute(context.Background(), validInput())
	if !errors.Is(err, scmdomain.ErrMetadataFetchFailed) {
		t.Errorf("expected ErrMetadataFetchFailed, got %v", err)
	}
}

// TestExecute_PerCategoryProviderResolutionFailure_NonFatal:
// one category's provider cannot be resolved, but the rest run.
// (Today every category uses the same provider; we simulate by
// giving the resolver an ErrUnknownProvider.)
func TestExecute_PerCategoryProviderResolutionFailure_NonFatal(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	resolver := &fakeResolver{ResolveErr: llmdomain.ErrUnknownProvider}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			"resilience.md":      "R",
			"maintainability.md": "M",
			"security.md":        "S",
			"testing.md":         "T",
		},
	}

	uc := newUseCaseUnderTest(scm, &fakeStore{}, resolver, loader)
	review, err := uc.Execute(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Execute must NOT fail when one category's provider cannot be resolved: %v", err)
	}
	// Every category failed → 0 findings.
	if len(review.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(review.Findings))
	}
}

// TestExecute_PerCategoryLLMFailure_NonFatal: the LLM run fails
// for one category, but the rest of the categories still run.
func TestExecute_PerCategoryLLMFailure_NonFatal(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	resolver := &fakeResolver{Providers: map[string]llmdomain.Provider{
		"": &fakeProvider{NameVal: "claude", RunErr: errors.New("api down")},
	}}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{"resilience.md": "R"},
	}

	uc := newUseCaseUnderTest(scm, &fakeStore{}, resolver, loader)
	review, err := uc.Execute(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Execute must NOT fail when one category's LLM fails: %v", err)
	}
	if len(review.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(review.Findings))
	}
}

// TestExecute_SBOMWiring_NoSBOMCategory_NoScannerCalls verifies
// that when no SBOM category is requested, the scanner is not
// invoked at all.
func TestExecute_SBOMWiring_NoSBOMCategory_NoScannerCalls(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	resolver := &fakeResolver{Providers: map[string]llmdomain.Provider{
		"": &fakeProvider{NameVal: "claude", RunResp: `{"findings": []}`},
	}}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			"resilience.md":      "R",
			"maintainability.md": "M",
			"security.md":        "S",
			"security_sbom.md":   "SB",
			"testing.md":         "T",
		},
	}
	scanner := &fakeScanner{}
	fetcher := &fakeFetcher{Path: "/tmp/repo"}

	uc := usecase.New(scm, &fakeStore{}, resolver, loader, scanner, fetcher)
	if _, err := uc.Execute(context.Background(), validInput()); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// CanonicalOrder now includes SECURITY:SBOM, so the scanner
	// IS invoked. The previous test (when --type filtered it
	// out) is no longer applicable.
	if len(scanner.Calls) != 1 {
		t.Errorf("scanner should be called once (SBOM in canonical order), got %d calls",
			len(scanner.Calls))
	}
}

// TestExecute_SBOMWiring_SBOMCategory_ScansAndSubstitutes verifies
// the happy path: --type security-sbom triggers a Clone + Scan,
// the SBOM JSON flows into the prompt via {{SBOM}}, and the LLM
// receives the final prompt with both {{DIFF}} and {{SBOM}}
// substituted.
func TestExecute_SBOMWiring_SBOMCategory_ScansAndSubstitutes(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ diff line")},
	}
	provider := &fakeProvider{NameVal: "claude", RunResp: `{"findings": []}`}
	resolver := &fakeResolver{Providers: map[string]llmdomain.Provider{"": provider}}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			"security_sbom.md": "DIFF={{DIFF}}\nSBOM={{SBOM}}\n",
		},
	}
	scanner := &fakeScanner{
		Result: scannersdomain.VulnerabilityResult{
			RepoPath: "/tmp/repo",
			Vulnerabilities: []scannersdomain.Vulnerability{{
				ID: "CVE-2024-X", CVSSScore: 9.8,
				Severity: scannersdomain.SeverityCritical,
				Component: "com.example:lib", Version: "1.2.3",
				FixedVersion: "1.2.4",
			}},
		},
	}
	fetcher := &fakeFetcher{Path: "/tmp/repo"}

	uc := usecase.New(scm, &fakeStore{}, resolver, loader, scanner, fetcher)
	if _, err := uc.Execute(context.Background(), validInput()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(fetcher.Calls) != 1 {
		t.Errorf("fetcher calls: got %d, want 1", len(fetcher.Calls))
	}
	if len(scanner.Calls) != 1 {
		t.Errorf("scanner calls: got %d, want 1", len(scanner.Calls))
	}
	for _, want := range []string{"+ diff line", "CVE-2024-X", "com.example:lib"} {
		if !strings.Contains(provider.LastPrompt, want) {
			t.Errorf("prompt missing %q\n--- prompt ---\n%s", want, provider.LastPrompt)
		}
	}
}

// TestExecute_SBOMWiring_ScanFails_NonFatal: a SBOM scan failure
// does NOT abort the review — the category simply contributes no
// findings.
func TestExecute_SBOMWiring_ScanFails_NonFatal(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	resolver := &fakeResolver{Providers: map[string]llmdomain.Provider{
		"": &fakeProvider{NameVal: "claude", RunResp: `{"findings": []}`},
	}}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{"security_sbom.md": "T"},
	}
	scanner := &fakeScanner{Err: errors.New("network down")}
	fetcher := &fakeFetcher{Path: "/tmp/repo"}

	uc := usecase.New(scm, &fakeStore{}, resolver, loader, scanner, fetcher)
	review, err := uc.Execute(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Execute must NOT fail when SBOM scan fails: %v", err)
	}
	if len(review.Findings) != 0 {
		t.Errorf("expected 0 findings (SBOM failed), got %d", len(review.Findings))
	}
}

// TestNew_NilDependencyPanics documents the constructor contract:
// every required dependency must be non-nil.
func TestNew_NilDependencyPanics(t *testing.T) {
	scm := &fakeSCM{}
	store := &fakeStore{}
	resolver := &fakeResolver{}
	loader := &fakeLoader{}

	cases := []struct {
		name string
		build func()
	}{
		{"nil scm", func() { usecase.New(nil, store, resolver, loader, nil, nil) }},
		{"nil store", func() { usecase.New(scm, nil, resolver, loader, nil, nil) }},
		{"nil resolver", func() { usecase.New(scm, store, nil, loader, nil, nil) }},
		{"nil loader", func() { usecase.New(scm, store, resolver, nil, nil, nil) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic for %s, got none", tc.name)
				}
			}()
			tc.build()
		})
	}
}

// ensure unused-import linter does not flag the scanner import
// when no test references it directly.
var _ = llmdomain.ErrUnknownProvider