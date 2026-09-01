package llm

import (
	"context"

	"github.com/LucasNav6/code-review-cli/helpers"
)

// codexProvider is the placeholder Provider for OpenAI's Codex CLI.
// The CLI itself is real (https://github.com/openai/codex) but this
// binary does not ship an adapter yet. Until then the provider is
// resolvable through New so the config command can store the choice,
// but every method returns ErrProviderNotImpl with a clear hint.
type codexProvider struct{}

// Codex returns the canonical Provider for the codex backend.
func Codex() Provider {
	return codexProvider{}
}

func (codexProvider) Name() string {
	return "codex"
}

func (codexProvider) ValidateInstalled(_ context.Context) error {
	return helpers.ErrProviderNotImpl
}

func (codexProvider) Run(_ context.Context, _ string) (string, error) {
	return "", helpers.ErrProviderNotImpl
}
