// Package ports declares the interfaces the llm bounded context
// consumes from the outside. The domain.Provider interface (see
// ../domain) describes what a Provider IS; this package describes
// what the use case needs from the LLM layer.
package ports

import "github.com/LucasNav6/code-review-cli/internal/llm/domain"

// Resolver turns a provider name into a concrete domain.Provider
// implementation. The lookup order (override > config > default)
// lives in the concrete implementation; this port only describes
// the contract.
//
// Why a separate Resolver instead of a factory in domain?
//
//  1. The factory needs access to config (to read the persisted
//     provider name) and to flag values (to honour --provider).
//     Both are external concerns the domain should not know about.
//  2. Tests can swap the Resolver for a static one that returns a
//     fake provider, so the use case never depends on real config.
//
// KnownNames() lets the cmd layer (and config validation) list
// what is available without binding to the concrete registry.
type Resolver interface {
	// Resolve returns the provider for the given name, or
	// domain.ErrUnknownProvider if no provider is registered
	// under that name. An empty name is treated as "use the
	// default" — implementations decide what the default is.
	Resolve(name string) (domain.Provider, error)

	// KnownNames returns the list of provider names that Resolve
	// can handle. Used by config validation and by the cmd layer
	// to build a useful error message when the user types a typo.
	KnownNames() []string
}