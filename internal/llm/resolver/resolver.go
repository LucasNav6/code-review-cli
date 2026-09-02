// Package resolver implements the ports.Resolver contract for the
// llm bounded context. It is the seam where adapters register
// themselves; everything outside this package talks to the
// resolver, never to a concrete adapter.
//
// This is NOT an adapter (it does not talk to the outside world); it
// is the port implementation. Putting it in its own package keeps
// the imports one-way: ports imports nothing of value here, and
// adapters do not know who is registering them.
package resolver

import (
	"fmt"

	"github.com/LucasNav6/code-review-cli/internal/llm/adapters/claude"
	"github.com/LucasNav6/code-review-cli/internal/llm/adapters/codex"
	"github.com/LucasNav6/code-review-cli/internal/llm/domain"
	"github.com/LucasNav6/code-review-cli/internal/llm/ports"
)

// registry maps a provider name to a constructor that returns the
// concrete implementation. Adding a new provider means adding one
// entry here — no other code in the codebase changes.
//
// Each entry is `name -> factory` rather than `name -> instance`
// so future stateful providers (e.g. one that caches an OAuth
// token) can be initialised lazily without breaking the registry
// contract.
var registry = map[string]func() domain.Provider{
	claude.Name: func() domain.Provider { return claude.New() },
	codex.Name:  func() domain.Provider { return codex.New() },
}

// Default is the provider name used when the user did not pick one.
// Today: claude. Future: this could be derived from config or env.
const Default = "claude"

// registryNames returns the sorted list of names the resolver
// knows. Order in the map is undefined in Go, so we sort for stable
// output.
func registryNames() []string {
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	// Simple insertion sort — the list is tiny (today: 2).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// Static implements ports.Resolver against the registry above. It
// is the production resolver — the one the cmd layer uses.
//
// The empty-name fallback is intentionally explicit: Resolve("")
// is the documented "use the default" case, not an error. This
// matches the rest of the codebase (see internal/config.KnownProvider,
// which treats empty as valid).
type Static struct{}

// New returns the production resolver. There is no constructor
// state today; the function exists so future refactors that need
// to inject configuration can do so without breaking call sites.
func New() ports.Resolver {
	return Static{}
}

// Resolve returns the provider for name, falling back to Default
// when name is empty. Returns domain.ErrUnknownProvider when the
// name is not in the registry (and not empty).
//
// We do NOT trim the input — a typo like "  claude  " should fail
// loud (because the user clearly meant something specific and we
// would be hiding their mistake). If you want "be lenient on
// whitespace", do that at the cmd layer where the value was read.
func (Static) Resolve(name string) (domain.Provider, error) {
	if name == "" {
		name = Default
	}
	factory, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q (known: %v)",
			domain.ErrUnknownProvider, name, registryNames())
	}
	return factory(), nil
}

// KnownNames returns the registry's names. Order is sorted so the
// caller can print a stable list.
func (Static) KnownNames() []string {
	return registryNames()
}