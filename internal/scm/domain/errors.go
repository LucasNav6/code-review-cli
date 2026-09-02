// Package domain owns the pure types, contracts and business rules
// of the "source control" bounded context.
//
// The context abstracts whatever tool the CLI uses to talk to the
// remote that hosts the pull request (today: the local `gh` CLI;
// tomorrow: `git`, an internal HTTP API, or a hosted SCM like
// GitLab). Anything that touches the outside world lives in
// ../adapters/; this package only knows about shapes and rules.
package domain

import "errors"

// Sentinel errors raised inside the scm context. They are declared
// here (not in helpers/) because they are part of the contract a
// future caller can rely on without depending on the helpers package.
//
// Cross-cutting sentinels (e.g. ErrEmptyURL) remain in helpers/.
var (
	// ErrSCMBinaryMissing is returned by Validate when the SCM
	// binary is not on PATH. Today this is `gh`; tomorrow it could
	// be `git` for self-hosted setups.
	ErrSCMBinaryMissing = errors.New("scm: required CLI is not installed")

	// ErrSCMBinaryInvalid is returned when the binary exists but
	// produces output that does not look like the expected tool
	// (e.g. a wrapper script that swallowed the real binary). The
	// detection is intentionally cheap (a version probe) so we can
	// fail fast before issuing the expensive commands.
	ErrSCMBinaryInvalid = errors.New("scm: CLI returned an unexpected response")

	// ErrSCMBinaryUnavailable is the catch-all when the binary is
	// present and looks right but the actual call fails for a
	// reason we cannot narrow down further (network, sandbox, etc.).
	ErrSCMBinaryUnavailable = errors.New("scm: CLI is unavailable")

	// ErrNotAuthenticated is returned when the SCM reports the user
	// is not logged in. The adapter decides how to detect this
	// (today: matching stderr strings from `gh`).
	ErrNotAuthenticated = errors.New("scm: not authenticated")

	// ErrPRNotFound is returned when the SCM can talk to the remote
	// but cannot resolve the supplied URL to a pull request. This is
	// distinct from ErrNotAuthenticated: the remote IS reachable, it
	// just does not know about this PR.
	ErrPRNotFound = errors.New("scm: pull request not found")

	// ErrDiffFetchFailed is the catch-all for diff fetch failures
	// that are not auth / not-found / network. The original stderr
	// is preserved via wrapping so the caller can show actionable
	// messages.
	ErrDiffFetchFailed = errors.New("scm: could not fetch pull request diff")

	// ErrMetadataFetchFailed is the metadata counterpart of
	// ErrDiffFetchFailed. Kept separate so the cmd layer can pick
	// the right ErrorType for logging without re-deriving which call
	// produced the failure.
	ErrMetadataFetchFailed = errors.New("scm: could not fetch pull request metadata")
)