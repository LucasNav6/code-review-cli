// Package llm is the legacy facade kept so existing callers
// (cmd/review.go, cmd/config.go) continue to compile while the
// bounded context lives in internal/llm/{domain,ports,adapters,…}.
//
// New code MUST NOT import this package. Use the bounded context
// directly:
//
//   - domain.Provider         — the contract an LLM backend satisfies.
//   - ports.Resolver          — the lookup mechanism (flag > config > default).
//   - internal/llm/resolver   — the production Resolver implementation.
//
// This package will be removed in H6 when cmd/review.go is migrated
// to the new use case.
package llm

import (
	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/llm/domain"
	"github.com/LucasNav6/code-review-cli/internal/llm/resolver"
)

// Provider is an alias for domain.Provider. Kept under the llm
// package so existing imports in cmd/ keep compiling during the
// refactor. Once cmd/review.go migrates to the new use case (H6),
// this alias goes away.
type Provider = domain.Provider

// ErrUnknownProvider is preserved for callers that match on it
// directly (cmd/config.go). The canonical sentinel lives in the llm
// domain package; the helpers package re-exports the same value
// because the existing import path is helpers.ErrUnknownProvider.
var ErrUnknownProvider = helpers.ErrUnknownProvider

// New resolves a Provider by name. Delegates to the production
// resolver. Kept as a package-level function (not a method) because
// existing call sites use it as `llm.New("claude")`.
func New(name string) (Provider, error) {
	return resolver.New().Resolve(name)
}

// KnownNames returns the provider names the resolver can handle.
// Same set the cmd/config command validates against.
func KnownNames() []string {
	return resolver.New().KnownNames()
}

// Compile-time check: the alias matches what adapters and the
// resolver return. The blank assignment below catches any drift
// between the alias and domain.Provider at build time.
var _ Provider = (domain.Provider)(nil)