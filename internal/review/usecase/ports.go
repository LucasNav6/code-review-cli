// Package usecase owns the ports (interfaces) the ReviewPRUseCase
// consumes. These are NOT the same as the per-bounded-context ports
// (e.g. internal/scm/ports.SCM, internal/llm/ports.Resolver):
// those describe what an adapter exposes. The ports in this file
// describe what THE USE CASE needs from the outside world to do its
// job, in the language of the review flow.
//
// Keeping them here (rather than importing them from each bounded
// context) means the use case has no upward dependencies in the
// package graph. Tests can wire fakes of these ports in one file
// without touching the adapters.
package usecase

import (
	"context"

	llmdomain "github.com/LucasNav6/code-review-cli/internal/llm/domain"
	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

// SCM is what the use case needs from the source-control layer.
// Today the only implementation is the gh adapter (see
// internal/scm/adapters/gh); tomorrow a git adapter can satisfy the
// same port without changes here.
//
// The use case does not care HOW the SCM is implemented — only that
// it can validate the binary, fetch metadata and fetch the diff.
// All three operations accept a context so the use case can cancel
// in-flight work when the cobra cmd-level deadline fires.
type SCM interface {
	Validate(ctx context.Context) error
	FetchMetadata(ctx context.Context, url scmdomain.PRURL) (scmdomain.PRMetadata, error)
	FetchDiff(ctx context.Context, url scmdomain.PRURL) (scmdomain.Diff, error)
}

// DiffStore persists a fetched diff so the use case can read it
// again later (e.g. to feed multiple LLM invocations without
// re-fetching). The concrete implementation is provided by the
// composition root (today: a FileStore under the OS cache dir).
//
// Re-exported from internal/scm/domain so callers do not have to
// import the scm package separately when wiring the use case.
// The use case itself does not call any method on it — it just
// holds the value and passes it through to Execute.
type DiffStore = scmdomain.DiffStore

// LLMProviderResolver turns a provider name into a concrete
// llmdomain.Provider. The use case calls this once per --provider
// override; the resolver handles the "override > config > default"
// lookup order. Keeping the port here (instead of importing the
// llm/ports.Resolver) means the use case does not have a transitive
// dependency on internal/llm/.
type LLMProviderResolver interface {
	Resolve(name string) (llmdomain.Provider, error)
}

// PromptLoader returns the raw template body for a given prompt
// file. The use case substitutes {{DIFF}} and sends the result to
// the LLM. We make this a port (not an import of the prompts
// adapter) so tests can return canned strings.
type PromptLoader interface {
	Load(promptFile reviewdomain.PromptFile) (string, error)
}

// OutputSink is the seam between the use case and whatever renders
// results for the user. The use case calls RenderHeader once and
// RenderReviewBlock once per category; the sink decides how to
// paint each block (lipgloss cards today, JSON tomorrow, HTML the
// day after).
//
// All methods take an io.Writer-style sink (passed in via the
// constructor) so the use case does not need to know about
// stdout/stderr or lipgloss.
type OutputSink interface {
	// RenderHeader paints the PR summary box.
	RenderHeader(meta scmdomain.PRMetadata)

	// RenderReviewBlock paints one category's review. The raw
	// response from the LLM is passed in; the sink is responsible
	// for parsing it into findings and rendering each one. We
	// keep parsing inside the sink (not the use case) so the use
	// case does not depend on the LLM response wire format.
	//
	// An error from RenderReviewBlock is non-fatal: the use case
	// logs it via the Notifier and continues with the next
	// category. This matches the documented "categories are
	// non-blocking" behaviour.
	RenderReviewBlock(category reviewdomain.Category, response string) error
}

// Notifier is the seam between the use case and whatever handles
// progress + error reporting. The use case does not know about
// stderr, logging, or spinners — it emits events and the
// notification adapter decides how to surface them.
//
// Warn is for non-fatal issues (a category failed, but the rest
// of the review continues). Error is reserved for issues the use
// case wants to surface but did not abort on. Fatal errors are
// returned from Execute; the caller is responsible for logging
// them.
type Notifier interface {
	Warn(msg string)
}