package domain

import "errors"

// Diff is the textual diff the SCM adapter produced for a pull
// request. The body is plain text in unified-diff format (the same
// shape `gh pr diff --patch` returns today). Adapters for other SCMs
// are expected to normalise to this format so the rest of the
// pipeline can be host-agnostic.
type Diff struct {
	// Body is the raw diff text. Callers may pass it verbatim to
	// prompt templates or persist it through a DiffStore.
	Body []byte
}

// ErrNoStoredDiff is returned by a DiffStore.Load when nothing was
// saved yet. It is its own sentinel (not io.EOF) because callers
// should match against the SCM context, not the stdlib.
var ErrNoStoredDiff = errors.New("scm: no diff has been stored yet")

// DiffStore is the persistence boundary for Diff values. Adapters
// own the concrete storage (today: a file under the OS cache dir;
// tomorrow: in-memory, S3, etc.) so the use case never touches the
// filesystem directly.
//
// The Save + Load pair lets the use case decouple "fetch the diff"
// from "feed the prompt": a single FetchDiff call can save once and
// many Load calls can read it back.
type DiffStore interface {
	// Save persists the diff bytes. Implementations decide where
	// and how (path, permissions, atomicity).
	Save(d Diff) error

	// Load returns the previously-saved diff. Returns
	// ErrNoStoredDiff if nothing has been saved yet, or an
	// implementation-specific error if the stored data is unreadable
	// (corruption, permission change, etc.).
	Load() (Diff, error)
}

// DiscardDiffStore is a no-op DiffStore useful for tests and for
// callers that do not want to persist the diff (e.g. a dry-run
// mode). Save returns nil, Load returns ErrNoStoredDiff.
type DiscardDiffStore struct{}

// Save discards the diff. Returns nil unconditionally.
func (DiscardDiffStore) Save(Diff) error { return nil }

// Load always returns ErrNoStoredDiff.
func (DiscardDiffStore) Load() (Diff, error) { return Diff{}, ErrNoStoredDiff }

// Compile-time check: DiscardDiffStore satisfies the DiffStore
// interface. Catches signature drift at build time.
var _ DiffStore = DiscardDiffStore{}