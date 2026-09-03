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
	scannersdomain "github.com/LucasNav6/code-review-cli/internal/scanners/domain"
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

// PromptContext carries the substitution values the use case
// passes to the prompt loader. Today two placeholders are
// supported:
//
//   - {{DIFF}}: the textual diff the LLM reviews. Mandatory for
//     every prompt (the use case asserts this before calling Load).
//   - {{SBOM}}: the JSON-encoded SBOM scan output. Only populated
//     for the security_sbom prompt; empty string for the others.
//
// Adding a new placeholder means adding a field here AND extending
// the fs.Loader + the use case's substitute call. Tests pin down
// every placeholder name.
type PromptContext struct {
	Diff string
	SBOM string
}

// PromptLoader returns the rendered prompt body for a given prompt
// file. The use case supplies a PromptContext with the substitution
// values; the loader is responsible for replacing every {{DIFF}}
// and {{SBOM}} it finds in the template.
//
// The loader owns NO business logic beyond string substitution. The
// use case owns the policy (when to substitute, what to put in).
type PromptLoader interface {
	Load(promptFile reviewdomain.PromptFile, ctx PromptContext) (string, error)
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
// Warn is for non-falable issues (a category failed, but the rest
// of the review continues). Error is reserved for issues the
// use case wants to surface but did not abort on. Fatal errors
// are returned from Execute; the caller is responsible for
// logging them.
type Notifier interface {
	Warn(msg string)
}

// SBOMScanner is the port the use case consumes to fetch
// CVE/vulnerability data from a local repo checkout. The contract
// is the same as internal/scanners/domain.SBOMScanner, redeclared
// here so the use case does not depend on the scanners package
// directly (keeps the bounded context graph acyclic).
type SBOMScanner interface {
	Scan(ctx context.Context, repoPath string) (scannersdomain.VulnerabilityResult, error)
}

// RepoFetcher is the port the use case consumes to clone a remote
// repository to a local directory before scanning it. The
// contract mirrors internal/scm/ports.RepoFetcher; redeclared
// here for the same decoupling reason as SBOMScanner.
type RepoFetcher interface {
	Clone(ctx context.Context, url scmdomain.PRURL) (string, error)
}