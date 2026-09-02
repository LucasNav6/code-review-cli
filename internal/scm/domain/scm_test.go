package domain_test

import (
	"errors"
	"testing"

	"github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

// TestNewPRURL_Accepts covers every shape we have seen in the wild.
// We accept the URL both with and without scheme so users can paste
// what they copied from the browser address bar.
func TestNewPRURL_Accepts(t *testing.T) {
	cases := []string{
		"https://github.com/owner/repo/pull/123",
		"github.com/owner/repo/pull/123",
		"https://github.com/owner/repo/pull/1",
		"https://github.com/owner-name/repo.name/pull/9999",
		"https://github.com/owner/repo/pull/123/files",     // suffix allowed
		"https://github.com/owner/repo/pull/123#issuecomment-1", // hash allowed
		"  https://github.com/owner/repo/pull/123  ",            // whitespace trimmed
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			got, err := domain.NewPRURL(in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() == "" {
				t.Fatal("NewPRURL returned empty string")
			}
		})
	}
}

// TestNewPRURL_Rejects pins down every shape we want to reject. The
// common failure mode is the user pasting the repo URL instead of
// the PR URL; that has to fail fast so we can show a useful error
// before issuing a network call.
func TestNewPRURL_Rejects(t *testing.T) {
	cases := []string{
		"",                                  // empty
		"   ",                               // whitespace only
		"https://github.com/owner/repo",     // repo URL, not PR
		"https://github.com/owner/repo/",    // trailing slash, no PR
		"https://github.com/owner/repo/issues/123", // issues, not PRs
		"https://github.com/owner/repo/pull/",      // missing number
		"https://github.com/owner/repo/pull/abc",    // non-numeric
		"not a url at all",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			_, err := domain.NewPRURL(in)
			if !errors.Is(err, domain.ErrInvalidPRURL) {
				t.Fatalf("expected ErrInvalidPRURL for %q, got %v", in, err)
			}
		})
	}
}

// TestPRURL_String ensures the Stringer-style method round-trips.
func TestPRURL_String(t *testing.T) {
	u, err := domain.NewPRURL("https://github.com/owner/repo/pull/123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.String() != "https://github.com/owner/repo/pull/123" {
		t.Fatalf("String() round-trip failed: got %q", u.String())
	}
}

// TestDiscardDiffStore satisfies the contract that
// DiscardDiffStore returns ErrNoStoredDiff from Load and nil from
// Save. Useful as a smoke test for the test fake that downstream
// packages will rely on.
func TestDiscardDiffStore(t *testing.T) {
	var store domain.DiffStore = domain.DiscardDiffStore{}

	if err := store.Save(domain.Diff{Body: []byte("+ foo := 1")}); err != nil {
		t.Fatalf("Save must return nil, got %v", err)
	}

	_, err := store.Load()
	if !errors.Is(err, domain.ErrNoStoredDiff) {
		t.Fatalf("Load must return ErrNoStoredDiff, got %v", err)
	}
}