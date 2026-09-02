// Package domain owns the pure types and contracts of the LLM
// bounded context: what a Provider IS (this file), the shape of a
// Response, and the sentinels the bounded context can raise.
//
// The Provider interface lives in domain (not in ports/) because it
// describes the nature of an LLM backend, not what a caller needs
// from it. Adapters implement it; the use case consumes it via
// higher-level ports (see ../ports).
package domain

import "context"

// Provider is the contract every LLM backend must satisfy.
//
// The review use case only sees this interface, never the concrete
// adapter, so swapping claude for codex (or a future gemini/openai)
// is a matter of wiring a different implementation in the
// composition root.
type Provider interface {
	// Name returns the stable identifier used in config
	// (e.g. "claude"). Empty strings are not valid.
	Name() string

	// ValidateInstalled probes the local CLI to make sure it is
	// both installed AND authenticated. Failures must be returned
	// as raw sentinels (see ./errors.go) so callers control how
	// they surface the failure. Adapters MUST NOT log here.
	ValidateInstalled(ctx context.Context) error

	// Run invokes the LLM with prompt and returns the captured
	// answer. Implementations are expected to trim leading and
	// trailing whitespace so downstream parsers can rely on the
	// shape. On any failure returns a raw sentinel — never logs.
	Run(ctx context.Context, prompt string) (string, error)
}