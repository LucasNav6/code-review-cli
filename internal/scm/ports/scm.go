// Package ports declares the interfaces the scm bounded context
// consumes from the outside. These are the seams where adapters
// (today: gh; tomorrow: git, hosted APIs) plug in.
//
// Anything that talks to a remote, a subprocess or the filesystem
// lives in ../adapters/. This package only describes what the use
// case needs.
package ports

import (
	"context"

	"github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

// SCM is the contract every source-control backend must satisfy.
// The review use case only sees this interface, never the concrete
// adapter, so swapping `gh` for `git` or a hosted SCM later is a
// matter of wiring a different implementation in the composition
// root.
//
// All methods accept a context so the caller (the use case, which
// in turn received it from cobra) can cancel in-flight subprocesses
// when the user hits Ctrl-C or the cmd-level deadline fires.
type SCM interface {
	// Validate checks that the underlying tool is installed and
	// usable. Implementations run a cheap probe (e.g. `gh --version`)
	// so the failure mode is fast and actionable. Must NOT log:
	// the caller decides how to surface the failure.
	Validate(ctx context.Context) error

	// FetchMetadata retrieves the small subset of PR fields the
	// header box needs. Returns domain.PRMetadata on success.
	FetchMetadata(ctx context.Context, url domain.PRURL) (domain.PRMetadata, error)

	// FetchDiff retrieves the unified-diff for the PR and returns
	// it as domain.Diff. The use case is responsible for
	// persisting it through a DiffStore if it needs to be read
	// again later (today the cmd layer does this via the cache
	// file).
	FetchDiff(ctx context.Context, url domain.PRURL) (domain.Diff, error)
}