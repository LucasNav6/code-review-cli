// Package usecase owns the ports (interfaces) the ReviewPRUseCase
// consumes from outside. The ports are minimal on purpose: the
// use case does not depend on cobra, lipgloss, or any concrete
// adapter. Tests wire fakes; production wires concrete adapters
// from internal/{scm,llm,prompts,scanners}.
//
// F3 (this commit) breaks the old OutputSink / Notifier contract:
// the use case no longer paints. It returns a domain.Review and
// the cmd / TUI layer decides how to display it. The remaining
// ports are the ones the use case actually needs to do its work.
package usecase

import (
	"context"

	llmdomain "github.com/LucasNav6/code-review-cli/internal/llm/domain"
	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	scannersdomain "github.com/LucasNav6/code-review-cli/internal/scanners/domain"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

// SCM is the contract every source-control backend must satisfy.
// The review use case consumes this interface, never the concrete
// adapter, so swapping gh for git or a hosted SCM later is a
// matter of wiring a different implementation in the composition
// root.
type SCM interface {
	Validate(ctx context.Context) error
	FetchMetadata(ctx context.Context, url scmdomain.PRURL) (scmdomain.PRMetadata, error)
	FetchDiff(ctx context.Context, url scmdomain.PRURL) (scmdomain.Diff, error)
}

// DiffStore persists a fetched diff so the use case can read it
// again later (e.g. to feed multiple LLM invocations without
// re-fetching). The concrete implementation is provided by the
// composition root.
type DiffStore interface {
	Save(d scmdomain.Diff) error
	Load() (scmdomain.Diff, error)
}

// PromptContext carries the substitution values the use case
// passes to the prompt loader.
type PromptContext struct {
	Diff string
	SBOM string
}

// PromptLoader returns the rendered prompt body for a given prompt
// file. The loader owns no business logic beyond string
// substitution; the use case owns the policy.
//
// Today the loader is a thin re-export of the fs adapter. We
// declare it in this package (not in internal/prompts) because
// keeping the type local avoids an awkward type conversion on
// every Load call. The fs adapter satisfies this interface
// directly because both use the same PromptFile + PromptContext
// types — no wrapper needed.
type PromptLoader interface {
	Load(promptFile reviewdomain.PromptFile, ctx PromptContext) (string, error)
}

// LLMProviderResolver turns a provider name into a concrete LLM
// provider implementation. Today an empty name means "use the
// default" (resolved via the persisted config).
//
// The interface re-declares llmdomain.Provider rather than
// aliasing it so the use case stays decoupled from the llm
// package. Today the llm/resolver package satisfies this
// interface directly (the method signatures match exactly).
type LLMProviderResolver interface {
	Resolve(name string) (llmdomain.Provider, error)
}

// SBOMScanner is the port the use case consumes to fetch
// CVE/vulnerability data from a local repo checkout.
type SBOMScanner interface {
	Scan(ctx context.Context, repoPath string) (scannersdomain.VulnerabilityResult, error)
}

// RepoFetcher is the port the use case consumes to clone a remote
// repository to a local directory before scanning it.
type RepoFetcher interface {
	Clone(ctx context.Context, url scmdomain.PRURL) (string, error)
}