// Package llm abstracts the local LLM CLI used for code review so the
// review command can switch between providers (claude today, codex in
// the future) without growing conditional code paths.
package llm

import (
	"context"

	"github.com/LucasNav6/code-review-cli/helpers"
)

// Provider is the contract every LLM backend must satisfy. The review
// command resolves a concrete provider through New and never touches
// the lower-level wrappers directly.
type Provider interface {
	// Name returns the stable identifier used in config (e.g. "claude").
	Name() string

	// ValidateInstalled probes the local CLI to make sure it is both
	// installed AND authenticated. Failures must be returned as raw
	// sentinels (see helpers package) so callers control logging.
	ValidateInstalled(ctx context.Context) error

	// Run invokes the CLI with prompt and returns the captured answer
	// trimmed of leading/trailing whitespace. On any failure it
	// returns a raw sentinel — never logs to stderr.
	Run(ctx context.Context, prompt string) (string, error)
}

// ErrUnknownProvider is returned by New when the configured name does
// not match any registered provider.
var ErrUnknownProvider = helpers.ErrUnknownProvider

// New resolves a Provider by name. It only knows about providers that
// have been registered through the package's init functions. The set
// of known names is: "claude" today, "codex" stub.
func New(name string) (Provider, error) {
	switch name {
	case "claude", "":
		return Claude(), nil
	case "codex":
		return Codex(), nil
	}
	return nil, ErrUnknownProvider
}

// KnownNames returns the provider names that New can resolve. The
// list is used by the config command to validate user input.
func KnownNames() []string {
	return []string{"claude", "codex"}
}
