package usecase_test

import (
	"context"

	llmdomain "github.com/LucasNav6/code-review-cli/internal/llm/domain"
	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
	"github.com/LucasNav6/code-review-cli/internal/review/usecase"
)

// All fakes in this file are designed for test use only. They
// expose every dependency's input (what was called, with what
// args) so tests can assert on the exact sequence of operations
// the use case performs. Production code MUST NOT depend on
// these types.

// fakeSCM implements SCM with configurable behaviour per method.
// Tests set the *Err fields to simulate failures and the *Out
// fields to control the success responses. Calls records every
// invocation so tests can assert on the call sequence.
type fakeSCM struct {
	ValidateErr     error
	FetchMetaErr    error
	FetchDiffErr    error
	ValidateCalls   int
	FetchMetaCalls  int
	FetchDiffCalls  int
	MetadataResult   scmdomain.PRMetadata
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
// Useful for asserting that the use case persists the diff it
// fetched before handing it to the prompt loader.
type fakeStore struct {
	Saved scmdomain.Diff
	// LoadResult is what Load returns (useful for tests that
	// want to simulate corruption or a missing diff).
	LoadResult scmdomain.Diff
	LoadErr    error
	SaveErr    error
	SavedCount int
}

func (f *fakeStore) Save(d scmdomain.Diff) error {
	f.Saved = d
	f.SavedCount++
	return f.SaveErr
}

func (f *fakeStore) Load() (scmdomain.Diff, error) {
	return f.LoadResult, f.LoadErr
}

// fakeResolver implements LLMProviderResolver. The provider it
// returns is configurable per name so tests can simulate "unknown
// provider" by leaving a name unmapped.
type fakeResolver struct {
	// Providers maps a name to the Provider returned by Resolve.
	// Unmapped names produce ResolveErr (or domain.ErrUnknownProvider
	// if ResolveErr is nil).
	Providers map[string]llmdomain.Provider
	ResolveErr error
	Calls      []string // every name that was resolved, in order
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

// fakeLoader implements PromptLoader by returning canned templates
// keyed by PromptFile. Useful for asserting that the use case picks
// the right prompt for each category.
type fakeLoader struct {
	// Templates maps the file name to the template body the loader
	// returns.
	Templates map[reviewdomain.PromptFile]string
	LoadErr   error
	Calls     []reviewdomain.PromptFile
	// LastContext captures the PromptContext from the most recent
	// Load call. Tests assert on this to verify the use case
	// passes the right SBOM/Diff values down.
	LastContext usecase.PromptContext
}

func (f *fakeLoader) Load(pf reviewdomain.PromptFile, ctx usecase.PromptContext) (string, error) {
	f.Calls = append(f.Calls, pf)
	f.LastContext = ctx
	if f.LoadErr != nil {
		return "", f.LoadErr
	}
	t, ok := f.Templates[pf]
	if !ok {
		return "", scmdomain.ErrSCMBinaryMissing // any sentinel will do
	}
	return t, nil
}

// fakeSink implements OutputSink by recording every call.
// RenderReviewBlock returns RenderErr so failures can be simulated.
type fakeSink struct {
	HeaderCalls        int
	HeaderMeta         scmdomain.PRMetadata
	BlockCalls         int
	BlockCallsByCategory map[reviewdomain.Category]string // cat -> response
	RenderErr          error
}

func (f *fakeSink) RenderHeader(meta scmdomain.PRMetadata) {
	f.HeaderCalls++
	f.HeaderMeta = meta
}

func (f *fakeSink) RenderReviewBlock(cat reviewdomain.Category, response string) error {
	f.BlockCalls++
	if f.BlockCallsByCategory == nil {
		f.BlockCallsByCategory = make(map[reviewdomain.Category]string)
	}
	f.BlockCallsByCategory[cat] = response
	return f.RenderErr
}

// fakeNotifier implements Notifier by capturing every warning.
type fakeNotifier struct {
	Warnings []string
}

func (f *fakeNotifier) Warn(msg string) {
	f.Warnings = append(f.Warnings, msg)
}

// fakeProvider implements llmdomain.Provider by returning canned
// responses and configurable errors.
type fakeProvider struct {
	NameVal          string
	ValidateErr      error
	RunResp          string
	RunErr           error
	ValidateCalls    int
	RunCalls         int
	LastPrompt       string
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