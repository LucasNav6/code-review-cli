package domain

import (
	"errors"
	"strings"
)

// PRURL is the canonical, validated shape of a pull-request URL the
// SCM adapter can resolve. Constructing one runs a cheap shape check
// so the rest of the package can assume the URL is non-empty and
// looks like a PR URL — domain code never re-validates.
//
// We deliberately keep this type loose: it does NOT verify the host
// or the PR number exist (that requires a network round-trip and
// belongs in the adapter). It only catches "the user typed garbage".
type PRURL string

// ErrInvalidPRURL is returned by NewPRURL when the input fails the
// cheap shape check. The check is intentionally minimal so the CLI
// gives the user a fast, actionable error before issuing a network
// call.
var ErrInvalidPRURL = errors.New("scm: not a valid pull request URL")

// NewPRURL trims whitespace, requires non-empty input, and returns a
// PRURL. The shape check looks for the "/pull/<number>" suffix so
// we can distinguish a PR URL from a generic repository URL — the
// latter is the most common user mistake today.
//
// Both "github.com/owner/repo/pull/123" and
// "https://github.com/owner/repo/pull/123" are accepted because users
// paste both forms.
func NewPRURL(raw string) (PRURL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrInvalidPRURL
	}

	// We accept anything that ends with "/pull/<digits>" — the SCM
	// adapter is the one that knows whether the host is supported.
	if !looksLikePRURL(trimmed) {
		return "", ErrInvalidPRURL
	}
	return PRURL(trimmed), nil
}

// String returns the URL as it was validated. Provided so callers
// can pass it to log lines or exec args without re-importing
// strconv.
func (u PRURL) String() string {
	return string(u)
}

// looksLikePRURL is a deliberately cheap check: any URL ending in
// "/pull/<digits>" with a non-empty prefix. We do not parse with
// net/url here because we want to keep this package free of
// dependencies and the check is good enough to catch the 99% case
// of "user typed a repo URL instead of a PR URL".
func looksLikePRURL(s string) bool {
	slashPull := "/pull/"
	idx := strings.LastIndex(s, slashPull)
	if idx < 0 {
		return false
	}
	tail := s[idx+len(slashPull):]
	if tail == "" {
		return false
	}
	// Require at least one digit; we do not validate the full number
	// range because the SCM will catch an out-of-range one.
	hasDigit := false
	for _, r := range tail {
		if r >= '0' && r <= '9' {
			hasDigit = true
			continue
		}
		// Anything after the digits (e.g. "/files", "#issuecomment-")
		// is allowed; we just need to have seen at least one digit.
		break
	}
	return hasDigit
}