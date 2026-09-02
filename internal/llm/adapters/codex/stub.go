// Package codex is the llm adapter for OpenAI's Codex CLI.
//
// Today the adapter is a stub: the CLI is real
// (https://github.com/openai/codex) but no real wrapper exists yet.
// The stub is intentionally resolvable through the registry so the
// config command can store the choice, but every method returns
// ErrProviderNotImpl so the user gets a clear "not implemented yet"
// signal instead of a confusing crash.
//
// When the real adapter is added it goes here, implementing
// domain.Provider — no other file in the codebase needs to change.
package codex

import (
	"context"

	"github.com/LucasNav6/code-review-cli/internal/llm/domain"
)

// Name is the stable identifier used in config.
const Name = "codex"

// Provider is the stub adapter. It satisfies domain.Provider so the
// resolver can register it, but every method returns
// ErrProviderNotImpl.
type Provider struct{}

// New returns the codex-backed domain.Provider.
func New() domain.Provider {
	return Provider{}
}

// Name returns the stable identifier.
func (Provider) Name() string { return Name }

// ValidateInstalled is unimplemented.
func (Provider) ValidateInstalled(_ context.Context) error {
	return domain.ErrProviderNotImpl
}

// Run is unimplemented.
func (Provider) Run(_ context.Context, _ string) (string, error) {
	return "", domain.ErrProviderNotImpl
}