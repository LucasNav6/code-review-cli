package gh

import (
	"errors"
	"testing"

	"github.com/LucasNav6/code-review-cli/internal/scm/domain"
	"github.com/LucasNav6/code-review-cli/internal/scm/ports"
)

// TestClassifyStderr pins down the mapping from `gh` stderr strings
// to scm-domain sentinels. Before the refactor the same function was
// duplicated in two places (internal/review/review.go and
// internal/pr/metadata.go). Today it lives once and is locked down
// by these cases.
//
// Each table row is one branch of the switch in classifyStderr.
// Adding a new branch requires adding the matching case here.
func TestClassifyStderr(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   error // sentinel to match; nil = no specific match expected
	}{
		// Authentication patterns.
		{
			name:   "not logged into",
			stderr: "gh: Not logged into any GitHub hosts. Run `gh auth login` to authenticate.",
			want:   domain.ErrNotAuthenticated,
		},
		{
			name:   "not authenticated",
			stderr: "ERROR: not authenticated. Please run `gh auth login`.",
			want:   domain.ErrNotAuthenticated,
		},
		{
			name:   "aborted you are not logged",
			stderr: "aborted: you are not logged into any GitHub hosts",
			want:   domain.ErrNotAuthenticated,
		},

		// PR-not-found patterns.
		{
			name:   "no pull requests found",
			stderr: "no pull requests found for repo owner/name",
			want:   domain.ErrPRNotFound,
		},
		{
			name:   "could not resolve to a repository",
			stderr: "GraphQL: Could not resolve to a Repository",
			want:   domain.ErrPRNotFound,
		},
		{
			name:   "plain not found",
			stderr: "pull request 9999 not found",
			want:   domain.ErrPRNotFound,
		},

		// Fallback: anything we cannot classify wraps the raw
		// stderr so the user still sees it.
		{
			name:   "garbage falls back",
			stderr: "some unexpected error from gh",
			want:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyStderr(tc.stderr)
			if tc.want == nil {
				// Fallback: should NOT be any of the documented
				// sentinels; it should be an opaque error that
				// still contains the original stderr for debug.
				if errors.Is(got, domain.ErrNotAuthenticated) ||
					errors.Is(got, domain.ErrPRNotFound) {
					t.Fatalf("fallback misclassified stderr %q: got %v", tc.stderr, got)
				}
				if !contains(got.Error(), tc.stderr) {
					t.Fatalf("fallback error %q must contain the original stderr %q", got.Error(), tc.stderr)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("classifyStderr(%q): got %v, want errors.Is(%v) == true",
					tc.stderr, got, tc.want)
			}
		})
	}
}

// TestClassifyStderr_CaseInsensitive documents that the classifier
// lower-cases the input before matching. Real gh output occasionally
// mixes casing across releases; we tolerate all of it.
func TestClassifyStderr_CaseInsensitive(t *testing.T) {
	if got := classifyStderr("NOT LOGGED INTO any GitHub hosts"); !errors.Is(got, domain.ErrNotAuthenticated) {
		t.Fatalf("expected ErrNotAuthenticated, got %v", got)
	}
}

// contains is a tiny helper that avoids dragging strings.Contains
// into a file that does not need it for anything else. Keeping the
// file self-contained also makes copy-paste of the test cases into
// a different package easier.
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestNewReturnsSCMInterface is the contract test: New() returns
// the port interface, never the concrete type, so callers can swap
// implementations freely.
func TestNewReturnsSCMInterface(t *testing.T) {
	scm := New()
	if scm == nil {
		t.Fatal("New() returned nil")
	}
	// Compile-time check that the returned value satisfies the
	// ports.SCM interface (see ../../ports/scm.go). The
	// assertion is on the assignment, not on a runtime method.
	var _ ports.SCM = scm
}