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
// fakes. Centralising the wiring here keeps each test focused on
// what it asserts.
//
// scanner and repoFetcher default to nil (no SBOM), keeping the
// existing test suite focused on the LLM flow. Tests that want
// SBOM exercise the wiring directly via usecase.New.
func newUseCaseUnderTest(
	scm usecase.SCM,
	store usecase.DiffStore,
	resolver usecase.LLMProviderResolver,
	loader usecase.PromptLoader,
	sink usecase.OutputSink,
	notifier usecase.Notifier,
) *usecase.ReviewPRUseCase {
	return usecase.New(scm, store, resolver, loader, sink, notifier, nil, nil)
}

// validInput returns a ReviewPRInput that would succeed on a happy
// path. Tests mutate it as needed.
func validInput() usecase.ReviewPRInput {
	return usecase.ReviewPRInput{
		URL:      "https://github.com/owner/repo/pull/123",
		ProviderOverride: "",
		ReviewType: reviewdomain.ReviewTypeAll,
	}
}

// TestExecute_HappyPath_AllCategories verifies that, given a
// working pipeline, the use case:
//  1. validates the URL,
//  2. validates the SCM,
//  3. fetches the diff and stores it,
//  4. fetches the metadata and renders the header,
//  5. resolves all four categories,
//  6. runs each category end-to-end (resolve provider, load
//     prompt, validate provider, run LLM, render block).
//
// Every step is verified via the fake's call counters. This is
// the test that proves the orchestration order is right.
func TestExecute_HappyPath_AllCategories(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 42, Title: "Test PR"},
		DiffResult:    scmdomain.Diff{Body: []byte("+ foo := 1")},
	}
	store := &fakeStore{}
	resolver := &fakeResolver{
		Providers: map[string]llmdomain.Provider{
			"": &fakeProvider{NameVal: "claude", RunResp: `{"findings": []}`},
		},
	}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			reviewdomain.PromptResilience:      "resilience template {{DIFF}}",
			reviewdomain.PromptMaintainability: "maintainability template {{DIFF}}",
			reviewdomain.PromptSecurity:        "security template {{DIFF}}",
			reviewdomain.PromptTesting:         "testing template {{DIFF}}",
		},
	}
	sink := &fakeSink{}
	notifier := &fakeNotifier{}

	uc := newUseCaseUnderTest(scm, store, resolver, loader, sink, notifier)
	if err := uc.Execute(context.Background(), validInput()); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	// Step 1: SCM.Validate called exactly once.
	if scm.ValidateCalls != 1 {
		t.Errorf("Validate calls: got %d, want 1", scm.ValidateCalls)
	}
	// Step 2: FetchDiff called once.
	if scm.FetchDiffCalls != 1 {
		t.Errorf("FetchDiff calls: got %d, want 1", scm.FetchDiffCalls)
	}
	// Step 3: Store.Save called once with the fetched diff.
	if store.SavedCount != 1 {
		t.Errorf("Save calls: got %d, want 1", store.SavedCount)
	}
	if string(store.Saved.Body) != "+ foo := 1" {
		t.Errorf("Saved diff: got %q, want %q", store.Saved.Body, "+ foo := 1")
	}
	// Step 4: FetchMetadata called once + header rendered.
	if scm.FetchMetaCalls != 1 {
		t.Errorf("FetchMetadata calls: got %d, want 1", scm.FetchMetaCalls)
	}
	if sink.HeaderCalls != 1 {
		t.Errorf("RenderHeader calls: got %d, want 1", sink.HeaderCalls)
	}
	if sink.HeaderMeta.Number != 42 {
		t.Errorf("Header meta: got %+v", sink.HeaderMeta)
	}
	// Step 5+6: Four categories were run.
	if sink.BlockCalls != 4 {
		t.Errorf("RenderReviewBlock calls: got %d, want 4", sink.BlockCalls)
	}
	// All four categories received the LLM response.
	for _, cat := range reviewdomain.CanonicalOrder() {
		if _, ok := sink.BlockCallsByCategory[cat]; !ok {
			t.Errorf("category %s was never rendered", cat)
		}
	}
	// All four prompt templates were loaded.
	if len(loader.Calls) != 4 {
		t.Errorf("Loader.Calls: got %d, want 4", len(loader.Calls))
	}
	// No warnings on a happy path.
	if len(notifier.Warnings) != 0 {
		t.Errorf("expected no warnings, got %v", notifier.Warnings)
	}
}

// TestExecute_HappyPath_SingleCategory runs the same flow with
// --type resilience and confirms only one category is executed.
func TestExecute_HappyPath_SingleCategory(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1, Title: "PR"},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	resolver := &fakeResolver{
		Providers: map[string]llmdomain.Provider{
			"": &fakeProvider{NameVal: "claude", RunResp: "{}"},
		},
	}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			reviewdomain.PromptResilience: "R {{DIFF}}",
		},
	}
	sink := &fakeSink{}

	uc := newUseCaseUnderTest(scm, &fakeStore{}, resolver, loader, sink, &fakeNotifier{})
	if err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL:      "https://github.com/owner/repo/pull/1",
		ReviewType: reviewdomain.ReviewTypeResilience,
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if sink.BlockCalls != 1 {
		t.Errorf("expected 1 block, got %d", sink.BlockCalls)
	}
	if _, ok := sink.BlockCallsByCategory[reviewdomain.CategoryResilience]; !ok {
		t.Error("resilience block missing")
	}
}

// TestExecute_URLShapeFailure is the "fail fast on typos" test.
// An invalid URL must abort before any SCM call is made — the
// user should not have to wait for a network round-trip to learn
// they pasted the repo URL.
func TestExecute_URLShapeFailure(t *testing.T) {
	scm := &fakeSCM{}
	uc := newUseCaseUnderTest(scm, &fakeStore{}, &fakeResolver{}, &fakeLoader{}, &fakeSink{}, &fakeNotifier{})

	err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL:      "https://github.com/owner/repo", // missing /pull/<n>
		ReviewType: reviewdomain.ReviewTypeAll,
	})
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
	if !errors.Is(err, scmdomain.ErrInvalidPRURL) {
		t.Errorf("expected ErrInvalidPRURL, got %v", err)
	}
	// No SCM call should have been made.
	if scm.ValidateCalls != 0 || scm.FetchDiffCalls != 0 || scm.FetchMetaCalls != 0 {
		t.Errorf("SCM was called despite URL failure: %+v", scm)
	}
}

// TestExecute_SCMValidateFailure verifies the first fatal in the
// pipeline: if gh is not installed, the use case stops right
// there.
func TestExecute_SCMValidateFailure(t *testing.T) {
	scm := &fakeSCM{ValidateErr: scmdomain.ErrSCMBinaryMissing}
	uc := newUseCaseUnderTest(scm, &fakeStore{}, &fakeResolver{}, &fakeLoader{}, &fakeSink{}, &fakeNotifier{})

	err := uc.Execute(context.Background(), validInput())
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
// failure aborts the entire review. There is nothing to review
// without the diff.
func TestExecute_DiffFetchFailureFatal(t *testing.T) {
	scm := &fakeSCM{FetchDiffErr: scmdomain.ErrDiffFetchFailed}
	sink := &fakeSink{}
	uc := newUseCaseUnderTest(scm, &fakeStore{}, &fakeResolver{}, &fakeLoader{}, sink, &fakeNotifier{})

	err := uc.Execute(context.Background(), validInput())
	if !errors.Is(err, scmdomain.ErrDiffFetchFailed) {
		t.Errorf("expected ErrDiffFetchFailed, got %v", err)
	}
	if sink.HeaderCalls != 0 {
		t.Error("Header must not render when diff fetch fails")
	}
}

// TestExecute_MetadataFetchFailureFatal: same logic for metadata.
func TestExecute_MetadataFetchFailureFatal(t *testing.T) {
	scm := &fakeSCM{
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
		FetchMetaErr: scmdomain.ErrMetadataFetchFailed,
	}
	sink := &fakeSink{}
	uc := newUseCaseUnderTest(scm, &fakeStore{}, &fakeResolver{}, &fakeLoader{}, sink, &fakeNotifier{})

	err := uc.Execute(context.Background(), validInput())
	if !errors.Is(err, scmdomain.ErrMetadataFetchFailed) {
		t.Errorf("expected ErrMetadataFetchFailed, got %v", err)
	}
}

// TestExecute_PerCategoryProviderResolutionFailure_NonFatal:
// failing to resolve a provider for one category must NOT abort
// the loop. The other categories must still run.
func TestExecute_PerCategoryProviderResolutionFailure_NonFatal(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	resolver := &fakeResolver{
		Providers: map[string]llmdomain.Provider{
			"claude": &fakeProvider{NameVal: "claude", RunResp: "{}"},
			// "codex" intentionally not registered → resolver
			// returns ErrUnknownProvider for it.
		},
	}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			reviewdomain.PromptResilience: "R {{DIFF}}",
			reviewdomain.PromptMaintainability: "M {{DIFF}}",
			reviewdomain.PromptSecurity:        "S {{DIFF}}",
			reviewdomain.PromptTesting:         "T {{DIFF}}",
		},
	}
	sink := &fakeSink{}
	notifier := &fakeNotifier{}

	uc := newUseCaseUnderTest(scm, &fakeStore{}, resolver, loader, sink, notifier)

	// No provider override → resolver returns the first registered
	// one. To force the unknown-provider branch we use an override
	// that does not exist.
	err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL:             "https://github.com/owner/repo/pull/1",
		ReviewType:      reviewdomain.ReviewTypeAll,
		ProviderOverride: "openai",
	})
	if err != nil {
		t.Fatalf("Execute must NOT fail when one category's provider cannot be resolved: %v", err)
	}
	// All four categories should have failed with a warning.
	if len(notifier.Warnings) != 4 {
		t.Errorf("expected 4 warnings, got %d: %v", len(notifier.Warnings), notifier.Warnings)
	}
	if sink.BlockCalls != 0 {
		t.Errorf("no block should have been rendered, got %d", sink.BlockCalls)
	}
}

// TestExecute_PerCategoryLLMFailure_NonFatal: one category's LLM
// run fails; the rest continue.
func TestExecute_PerCategoryLLMFailure_NonFatal(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	// We use --type security so only one category runs. The
	// provider for security fails on Run; we expect a warning
	// and Execute to return nil.
	resolver := &fakeResolver{
		Providers: map[string]llmdomain.Provider{
			"claude": &fakeProvider{NameVal: "claude", RunErr: errors.New("api down")},
		},
	}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			reviewdomain.PromptSecurity: "S {{DIFF}}",
		},
	}
	sink := &fakeSink{}
	notifier := &fakeNotifier{}

	uc := newUseCaseUnderTest(scm, &fakeStore{}, resolver, loader, sink, notifier)
	err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL:             "https://github.com/owner/repo/pull/1",
		ReviewType:      reviewdomain.ReviewTypeSecurity,
		ProviderOverride: "claude",
	})
	if err != nil {
		t.Fatalf("Execute must NOT fail when one category's LLM fails: %v", err)
	}
	if len(notifier.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(notifier.Warnings))
	}
	if sink.BlockCalls != 0 {
		t.Errorf("no block should have been rendered, got %d", sink.BlockCalls)
	}
}

// TestExecute_PromptSubstitution_PlaceholderIsFilled verifies that
// the {{DIFF}} placeholder is replaced with the diff body before
// the prompt is handed to the LLM.
func TestExecute_PromptSubstitution_PlaceholderIsFilled(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("DIFF_BODY_HERE")},
	}
	provider := &fakeProvider{NameVal: "claude", RunResp: "{}"}
	resolver := &fakeResolver{Providers: map[string]llmdomain.Provider{
		"claude": provider,
	}}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			reviewdomain.PromptResilience: "BEFORE {{DIFF}} AFTER",
		},
	}

	uc := newUseCaseUnderTest(scm, &fakeStore{}, resolver, loader, &fakeSink{}, &fakeNotifier{})
	err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL:             "https://github.com/owner/repo/pull/1",
		ReviewType:      reviewdomain.ReviewTypeResilience,
		ProviderOverride: "claude",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	want := "BEFORE DIFF_BODY_HERE AFTER"
	if provider.LastPrompt != want {
		t.Errorf("prompt sent to LLM:\n got  %q\n want %q",
			provider.LastPrompt, want)
	}
}

// TestExecute_PromptSubstitution_PlaceholderMissingAppends covers
// the documented "append if placeholder is missing" behaviour.
// Templates that forgot {{DIFF}} still get the diff — appended at
// the end so the LLM sees it.
func TestExecute_PromptSubstitution_PlaceholderMissingAppends(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("DIFF_BODY_HERE")},
	}
	provider := &fakeProvider{NameVal: "claude", RunResp: "{}"}
	resolver := &fakeResolver{Providers: map[string]llmdomain.Provider{
		"claude": provider,
	}}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			reviewdomain.PromptResilience: "no placeholder here",
		},
	}

	uc := newUseCaseUnderTest(scm, &fakeStore{}, resolver, loader, &fakeSink{}, &fakeNotifier{})
	err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL:             "https://github.com/owner/repo/pull/1",
		ReviewType:      reviewdomain.ReviewTypeResilience,
		ProviderOverride: "claude",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(provider.LastPrompt, "no placeholder here") {
		t.Errorf("template body missing: %q", provider.LastPrompt)
	}
	if !strings.Contains(provider.LastPrompt, "DIFF_BODY_HERE") {
		t.Errorf("diff body missing from appended prompt: %q", provider.LastPrompt)
	}
}

// TestNew_NilDependencyPanics documents the "all dependencies are
// required" contract. A use case built with any nil port is a
// programming error and must fail loudly at construction time,
// not later at runtime when the nil port is dereferenced.
func TestNew_NilDependencyPanics(t *testing.T) {
	scm := &fakeSCM{}
	store := &fakeStore{}
	resolver := &fakeResolver{}
	loader := &fakeLoader{}
	sink := &fakeSink{}
	notifier := &fakeNotifier{}

	cases := []struct {
		name string
		build func()
	}{
{"nil scm", func() { usecase.New(nil, store, resolver, loader, sink, notifier, nil, nil) }},
	{"nil store", func() { usecase.New(scm, nil, resolver, loader, sink, notifier, nil, nil) }},
	{"nil resolver", func() { usecase.New(scm, store, nil, loader, sink, notifier, nil, nil) }},
	{"nil loader", func() { usecase.New(scm, store, resolver, nil, sink, notifier, nil, nil) }},
	{"nil sink", func() { usecase.New(scm, store, resolver, loader, nil, notifier, nil, nil) }},
	{"nil notifier", func() { usecase.New(scm, store, resolver, loader, sink, nil, nil, nil) }},
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

// TestExecute_UnknownReviewTypeFatal ensures that an unknown
// --type value aborts the whole review. Running a zero-category
// review would silently produce no output, which is worse than a
// clear error.
func TestExecute_UnknownReviewTypeFatal(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	sink := &fakeSink{}
	uc := newUseCaseUnderTest(scm, &fakeStore{}, &fakeResolver{}, &fakeLoader{}, sink, &fakeNotifier{})

	err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL:             "https://github.com/owner/repo/pull/1",
		ReviewType:      reviewdomain.ReviewType("bogus"),
		ProviderOverride: "",
	})
	if err == nil {
		t.Fatal("expected error for unknown --type, got nil")
	}
	if !errors.Is(err, reviewdomain.ErrUnknownReviewType) {
		t.Errorf("expected ErrUnknownReviewType, got %v", err)
	}
	// No blocks should have been rendered.
	if sink.BlockCalls != 0 {
		t.Errorf("expected 0 blocks, got %d", sink.BlockCalls)
	}
}

// TestExecute_ProviderOverridePassedToResolver verifies that the
// value of --provider flows through to the resolver unchanged.
// This is the contract cmd/review.go will rely on once H6 wires
// it up.
func TestExecute_ProviderOverridePassedToResolver(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	resolver := &fakeResolver{
		Providers: map[string]llmdomain.Provider{
			"codex": &fakeProvider{NameVal: "codex", RunResp: "{}"},
		},
	}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			reviewdomain.PromptResilience: "R {{DIFF}}",
		},
	}
	uc := newUseCaseUnderTest(scm, &fakeStore{}, resolver, loader, &fakeSink{}, &fakeNotifier{})

	err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL:             "https://github.com/owner/repo/pull/1",
		ReviewType:      reviewdomain.ReviewTypeResilience,
		ProviderOverride: "codex",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// The resolver must have been called with "codex".
	if len(resolver.Calls) == 0 || resolver.Calls[0] != "codex" {
		t.Errorf("resolver was not called with override: %v", resolver.Calls)
	}
}

// fakeScanner implements usecase.SBOMScanner for the SBOM wiring
// tests below. Returns canned vulnerabilities on demand.
type fakeScanner struct {
	Result scannersdomain.VulnerabilityResult
	Err    error
	Calls  []string // every repoPath passed to Scan
}

func (f *fakeScanner) Scan(ctx context.Context, repoPath string) (scannersdomain.VulnerabilityResult, error) {
	f.Calls = append(f.Calls, repoPath)
	return f.Result, f.Err
}

// fakeFetcher implements usecase.RepoFetcher. Returns a stable
// tmpdir path for assertions; the use case does not use the path
// except to hand it to the scanner.
type fakeFetcher struct {
	Path string
	Err  error
	Calls []scmdomain.PRURL
}

func (f *fakeFetcher) Clone(ctx context.Context, url scmdomain.PRURL) (string, error) {
	f.Calls = append(f.Calls, url)
	return f.Path, f.Err
}

// TestExecute_SBOMWiring_NoSBOMCategory_NoScannerCalls verifies that
// when the user does NOT request the SBOM category, the scanner is
// not invoked at all. The use case skips buildSBOMContext entirely
// when no category needs it.
func TestExecute_SBOMWiring_NoSBOMCategory_NoScannerCalls(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	resolver := &fakeResolver{
		Providers: map[string]llmdomain.Provider{
			"claude": &fakeProvider{NameVal: "claude", RunResp: "{}"},
		},
	}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			reviewdomain.PromptResilience: "R {{DIFF}}",
		},
	}
	scanner := &fakeScanner{}
	fetcher := &fakeFetcher{Path: "/tmp/repo"}

	uc := usecase.New(scm, &fakeStore{}, resolver, loader, &fakeSink{}, &fakeNotifier{},
		scanner, fetcher)
	if err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL:             "https://github.com/owner/repo/pull/1",
		ReviewType:      reviewdomain.ReviewTypeResilience, // NOT security-sbom
		ProviderOverride: "claude",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(scanner.Calls) != 0 {
		t.Errorf("scanner must NOT be called when no SBOM category requested, got %d calls",
			len(scanner.Calls))
	}
	if len(fetcher.Calls) != 0 {
		t.Errorf("fetcher must NOT be called either, got %d calls", len(fetcher.Calls))
	}
}

// TestExecute_SBOMWiring_SBOMCategory_ScansAndSubstitutes verifies
// the happy path: --type security-sbom triggers a Clone + Scan,
// the SBOM data flows into the prompt via {{SBOM}}, and the LLM
// receives the final prompt with both {{DIFF}} and {{SBOM}}
// substituted.
func TestExecute_SBOMWiring_SBOMCategory_ScansAndSubstitutes(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ diff line")},
	}
	provider := &fakeProvider{NameVal: "claude", RunResp: `{"findings": []}`}
	resolver := &fakeResolver{
		Providers: map[string]llmdomain.Provider{"claude": provider},
	}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			reviewdomain.PromptSecuritySBOM: "DIFF={{DIFF}}\nSBOM={{SBOM}}\n",
		},
	}
	scanner := &fakeScanner{
		Result: scannersdomain.VulnerabilityResult{
			RepoPath: "/tmp/repo",
			Vulnerabilities: []scannersdomain.Vulnerability{{
				ID:       "CVE-2024-X",
				CVSSScore: 9.8,
				Severity:  scannersdomain.SeverityCritical,
				Component: "com.example:lib",
				Version:   "1.2.3",
				FixedVersion: "1.2.4",
			}},
		},
	}
	fetcher := &fakeFetcher{Path: "/tmp/repo"}

	uc := usecase.New(scm, &fakeStore{}, resolver, loader, &fakeSink{}, &fakeNotifier{},
		scanner, fetcher)
	if err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL:             "https://github.com/owner/repo/pull/1",
		ReviewType:      reviewdomain.ReviewTypeSecuritySBOM,
		ProviderOverride: "claude",
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Fetcher + scanner were each called exactly once.
	if len(fetcher.Calls) != 1 {
		t.Errorf("fetcher calls: got %d, want 1", len(fetcher.Calls))
	}
	if len(scanner.Calls) != 1 {
		t.Errorf("scanner calls: got %d, want 1", len(scanner.Calls))
	}

	// The prompt handed to the LLM must contain BOTH the diff
	// body AND the SBOM JSON (proves both substitutions fired).
	wantParts := []string{"+ diff line", "CVE-2024-X", "com.example:lib"}
	for _, want := range wantParts {
		if !strings.Contains(provider.LastPrompt, want) {
			t.Errorf("prompt missing %q\n--- prompt ---\n%s", want, provider.LastPrompt)
		}
	}
}

// TestExecute_SBOMWiring_ScanFails_NonFatal verifies that a SBOM
// scan failure does NOT abort the whole review — the category
// emits a warning and the user gets the rest of the output.
func TestExecute_SBOMWiring_ScanFails_NonFatal(t *testing.T) {
	scm := &fakeSCM{
		MetadataResult: scmdomain.PRMetadata{Number: 1},
		DiffResult:    scmdomain.Diff{Body: []byte("+ x")},
	}
	resolver := &fakeResolver{
		Providers: map[string]llmdomain.Provider{
			"claude": &fakeProvider{NameVal: "claude", RunResp: "{}"},
		},
	}
	loader := &fakeLoader{
		Templates: map[reviewdomain.PromptFile]string{
			reviewdomain.PromptSecuritySBOM: "template {{DIFF}}",
		},
	}
	scanner := &fakeScanner{Err: errors.New("network down")}
	fetcher := &fakeFetcher{Path: "/tmp/repo"}
	notifier := &fakeNotifier{}

	uc := usecase.New(scm, &fakeStore{}, resolver, loader, &fakeSink{}, notifier,
		scanner, fetcher)
	err := uc.Execute(context.Background(), usecase.ReviewPRInput{
		URL:             "https://github.com/owner/repo/pull/1",
		ReviewType:      reviewdomain.ReviewTypeSecuritySBOM,
		ProviderOverride: "claude",
	})
	if err != nil {
		t.Fatalf("Execute must NOT fail when SBOM scan fails: %v", err)
	}
	// At least one warning must mention the SBOM skip.
	foundSBOMWarn := false
	for _, w := range notifier.Warnings {
		if strings.Contains(w, "SBOM") {
			foundSBOMWarn = true
			break
		}
	}
	if !foundSBOMWarn {
		t.Errorf("expected SBOM warning, got %v", notifier.Warnings)
	}
}