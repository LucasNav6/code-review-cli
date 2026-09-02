// Package gh implements the SCM port against the local `gh` CLI.
//
// This file is the SINGLE source of truth for mapping the stderr of
// `gh` invocations to scm-domain sentinels. Before this refactor the
// same function lived in two places (internal/review/review.go and
// internal/pr/metadata.go) — keeping them in sync was a manual
// chore and a real drift hazard. The duplication is gone for good.
package gh

import (
	"errors"
	"strings"

	"github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

// classifyStderr maps the stderr of a failed `gh` call to an scm
// sentinel. Callers wrap the result with context (the URL, the call
// that produced it, etc.) before surfacing it to the user.
//
// The matching is intentionally narrow and based on substrings the
// gh CLI has emitted for years. If gh ever changes its wording, the
// worst case is that we fall back to a wrapped error containing the
// original stderr — the user still sees the message.
//
// classifyStderr is unexported because the port callers should not
// reach into gh internals: they receive already-classified sentinels
// from FetchDiff / FetchMetadata / Validate. It lives in this file
// (not in adapter.go) so both Validate and the Fetch* methods can
// call it without depending on adapter.go internals.
func classifyStderr(stderr string) error {
	message := strings.ToLower(stderr)

	switch {
	case strings.Contains(message, "not logged into") ||
		strings.Contains(message, "not authenticated") ||
		strings.Contains(message, "aborted: you are not logged"):
		return domain.ErrNotAuthenticated

	case strings.Contains(message, "no pull requests found") ||
		strings.Contains(message, "could not resolve to a repository") ||
		strings.Contains(message, "not found"):
		return domain.ErrPRNotFound
	}

	// No specific match — the caller will wrap this with the
	// appropriate "what were we trying to fetch" context.
	return errors.New("unclassified gh error: " + strings.TrimSpace(stderr))
}