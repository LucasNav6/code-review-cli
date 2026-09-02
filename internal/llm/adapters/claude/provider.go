// Package claude implements the LLM port against the local `claude`
// CLI. The package is self-contained: it owns the subprocess
// invocation (Run), the install/auth probe (ValidateInstalled), and
// the adapter that wires both behind the domain.Provider interface.
package claude

import (
	"context"

	"github.com/LucasNav6/code-review-cli/internal/llm/domain"
)

// Name is the stable identifier used in config (and in KnownNames)
// to select this provider. Keep it lowercase to match the
// convention in internal/config.
const Name = "claude"

// Provider is the adapter that exposes the local `claude` CLI as a
// domain.Provider. It is returned by New() so callers do not bind
// to the concrete struct.
type Provider struct{}

// New returns a claude-backed domain.Provider. Safe to call from any
// composition root; no initialisation needed because the claude CLI
// is queried lazily on Validate / Run.
func New() domain.Provider {
	return Provider{}
}

// Name returns the stable identifier used in config.
func (Provider) Name() string { return Name }

// ValidateInstalled probes the local `claude` CLI by delegating to
// the package-level ValidateInstalled helper. The split between
// the helper and the method exists because ValidateInstalled is
// also useful at startup (e.g. in cmd/config.go) where the caller
// does not yet hold a Provider.
func (Provider) ValidateInstalled(ctx context.Context) error {
	return ValidateInstalled(ctx)
}

// Run invokes the `claude` CLI with prompt and returns the captured
// answer. Delegates to the package-level Run helper, which owns the
// subprocess mechanics and error classification.
func (p Provider) Run(ctx context.Context, prompt string) (string, error) {
	return Run(ctx, prompt)
}