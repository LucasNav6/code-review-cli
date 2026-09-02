// Package helpers hosts cross-domain concerns that do not belong to
// any single <domain> package: shared sentinel errors, the dispatcher
// from sentinel to actionable hint, and any other generic helper that
// two or more contexts (review, update, …) might consume.
package helpers

import (
	"errors"
)

// Sentinel errors raised by every context. They are declared here
// (rather than inside each context package) so the dispatcher below
// can match against a single set.
var (
	// Review-related.
	ErrEmptyURL          = errors.New("pull request URL is required")
	ErrCommandFlag       = errors.New("could not read command flag")
	ErrNotGitHubURL      = errors.New("URL must belong to github.com")
	ErrNotPullRequestURL = errors.New("URL is not a pull request")
	ErrInvalidPRNumber   = errors.New("invalid pull request number")

	// gh CLI / network.
	ErrGitHubCLINotInstalled = errors.New(`GitHub CLI ("gh") is not installed`)
	ErrGitHubCLIUnavailable  = errors.New(`GitHub CLI ("gh") is unavailable`)
	ErrGitHubCLIInvalid      = errors.New(`GitHub CLI ("gh") returned an unexpected response`)
	ErrGHNotAuthenticated    = errors.New(`GitHub CLI ("gh") is not authenticated`)
	ErrPRNotFound            = errors.New("pull request not found on the remote")
	ErrMetadataFetchFailed   = errors.New("could not fetch pull request metadata")
	ErrDiffFetchFailed       = errors.New("could not fetch pull request diff")
	ErrDiffStoreFailed       = errors.New("could not store pull request diff")

	// claude CLI.
	ErrClaudeNotInstalled     = errors.New(`Claude CLI ("claude") is not installed`)
	ErrClaudeUnavailable      = errors.New(`Claude CLI ("claude") is unavailable`)
	ErrClaudeNotAuthenticated = errors.New(`Claude CLI ("claude") is not authenticated`)
	ErrClaudeTimeout          = errors.New(`Claude CLI ("claude") timed out`)
	ErrClaudeInvalid          = errors.New(`Claude CLI ("claude") returned an unexpected response`)

	// claudereview parsing.
	ErrNoFindingsSection = errors.New("claude response contained no `## Hallazgo` section")
	ErrMalformedFinding  = errors.New("claude response had a Hallazgo block with no `archivo` field")
	ErrHunkNotFound      = errors.New("no diff hunk covers the line reported by claude")

	// config + llm.
	ErrUnknownProvider    = errors.New("unknown LLM provider")
	ErrProviderNotImpl    = errors.New("LLM provider is not implemented yet")
	ErrInvalidConfigValue = errors.New("invalid configuration value")
	ErrConfigNotFound     = errors.New("configuration key not set")

	// claude response.
	ErrEmptyResponse     = errors.New("claude returned an empty response")
	ErrMalformedResponse = errors.New("claude response was not valid JSON")
)
