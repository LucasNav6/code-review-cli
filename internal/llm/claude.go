package llm

import (
	"context"

	claudecmd "github.com/LucasNav6/code-review-cli/internal/claude"
)

// claudeProvider is the Provider implementation backed by the local
// claude CLI. It is constructed through Claude() so future callers do
// not depend on the concrete type.
type claudeProvider struct{}

// Claude returns the canonical Provider for the claude backend.
func Claude() Provider {
	return claudeProvider{}
}

func (claudeProvider) Name() string {
	return "claude"
}

func (claudeProvider) ValidateInstalled(ctx context.Context) error {
	return claudecmd.ValidateInstalled(ctx)
}

func (claudeProvider) Run(ctx context.Context, prompt string) (string, error) {
	return claudecmd.Run(ctx, prompt)
}
