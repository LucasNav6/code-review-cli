package usecase_test

import (
	"context"

	llmdomain "github.com/LucasNav6/code-review-cli/internal/llm/domain"
	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	scannersdomain "github.com/LucasNav6/code-review-cli/internal/scanners/domain"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
	"github.com/LucasNav6/code-review-cli/internal/review/usecase"
)

// All fakes in this file are designed for test use only. They
// expose every dependency's input (what was called, with what
// args) so tests can assert on the exact sequence of operations
// the use case performs. Production code MUST NOT depend on
// these types.

// fakeSCM implements usecase.SCM. Tests configure its fields to
// return canned responses or simulate failures.
type fakeSCM struct {
	ValidateErr     error
	FetchMetaErr    error
	FetchDiffErr    error
	ValidateCalls   int
	FetchMetaCalls  int
	FetchDiffCalls  int
	MetadataResult  scmdomain.PRMetadata
	DiffResult      scmdomain.Diff
}

func (f *fakeSCM) Validate(ctx context.Context) error {
	f.ValidateCalls++
	return f.ValidateErr
}

func (f *fakeSCM) FetchMetadata(ctx context.Context, url scmdomain.PRURL) (scmdomain.PRMetadata, error) {
	f.FetchMetaCalls++
	return f.MetadataResult, f.FetchMetaErr
}

func (f *fakeSCM) FetchDiff(ctx context.Context, url scmdomain.PRURL) (scmdomain.Diff, error) {
	f.FetchDiffCalls++
	return f.DiffResult, f.FetchDiffErr
}

// fakeStore implements DiffStore by holding the diff in memory.
type fakeStore struct {
	Saved     scmdomain.Diff
	LoadResult scmdomain.Diff
	LoadErr   error
	SaveErr   error
}

func (s *fakeStore) Save(d scmdomain.Diff) error {
	s.Saved = d
	return s.SaveErr
}

func (s *fakeStore) Load() (scmdomain.Diff, error) {
	return s.LoadResult, s.LoadErr
}

// fakeResolver implements LLMProviderResolver. Tests map provider
// names to the Provider returned by Resolve.
type fakeResolver struct {
	Providers  map[string]llmdomain.Provider
	ResolveErr error
	Calls      []string
}

func (f *fakeResolver) Resolve(name string) (llmdomain.Provider, error) {
	f.Calls = append(f.Calls, name)
	if f.ResolveErr != nil {
		return nil, f.ResolveErr
	}
	p, ok := f.Providers[name]
	if !ok {
		return nil, llmdomain.ErrUnknownProvider
	}
	return p, nil
}

// fakeProvider implements llmdomain.Provider with canned responses.
type fakeProvider struct {
	NameVal       string
	ValidateErr   error
	RunResp       string
	RunErr        error
	ValidateCalls int
	RunCalls      int
	LastPrompt    string
}

func (f *fakeProvider) Name() string { return f.NameVal }

func (f *fakeProvider) ValidateInstalled(ctx context.Context) error {
	f.ValidateCalls++
	return f.ValidateErr
}

func (f *fakeProvider) Run(ctx context.Context, prompt string) (string, error) {
	f.RunCalls++
	f.LastPrompt = prompt
	return f.RunResp, f.RunErr
}

// fakeLoader implements PromptLoader. Templates is keyed by the
// prompt file so tests can return a different body per category.
type fakeLoader struct {
	Templates map[reviewdomain.PromptFile]string
	LoadErr   error
	Calls     []reviewdomain.PromptFile
	LastCtx   usecase.PromptContext
}

func (f *fakeLoader) Load(promptFile reviewdomain.PromptFile, ctx usecase.PromptContext) (string, error) {
	f.Calls = append(f.Calls, promptFile)
	f.LastCtx = ctx
	if f.LoadErr != nil {
		return "", f.LoadErr
	}
	t, ok := f.Templates[promptFile]
	if !ok {
		return "", scmdomain.ErrSCMBinaryMissing
	}
	return t, nil
}

// fakeScanner implements SBOMScanner with canned results.
type fakeScanner struct {
	Result scannersdomain.VulnerabilityResult
	Err    error
	Calls  []string
}

func (f *fakeScanner) Scan(ctx context.Context, repoPath string) (scannersdomain.VulnerabilityResult, error) {
	f.Calls = append(f.Calls, repoPath)
	return f.Result, f.Err
}

// fakeFetcher implements RepoFetcher. Returns Path on every call.
type fakeFetcher struct {
	Path  string
	Err   error
	Calls []scmdomain.PRURL
}

func (f *fakeFetcher) Clone(ctx context.Context, url scmdomain.PRURL) (string, error) {
	f.Calls = append(f.Calls, url)
	return f.Path, f.Err
}

// ensure reviewdomain stays referenced (used elsewhere in tests).
var _ = reviewdomain.CategoryResilience