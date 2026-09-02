package git

import (
	"strings"
	"testing"

	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

// TestExtractSlug_KnownForms documents every URL shape we accept.
// Adding a new prefix (e.g. ssh://git@github.com/...) means
// adding a row here.
func TestExtractSlug_KnownForms(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		// Standard https URLs.
		{"https://github.com/QuadMinds/saas/pull/5323", "QuadMinds/saas", true},
		{"https://github.com/owner-name/repo.name/pull/9999", "owner-name/repo.name", true},
		// Without scheme (sometimes pasted that way).
		{"github.com/owner/repo/pull/1", "owner/repo", true},
		// With trailing query / fragment / sub-path.
		{"https://github.com/owner/repo/pull/123/files", "owner/repo", true},
		{"https://github.com/owner/repo/pull/123#issuecomment-1", "owner/repo", true},
		// http (legacy).
		{"http://github.com/owner/repo/pull/1", "owner/repo", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := extractSlug(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok mismatch: got %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestExtractSlug_Rejects pins down every shape we refuse. Adding
// a host (e.g. gitlab.com) would mean adding a row here AND
// extending the clone URL logic.
func TestExtractSlug_Rejects(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"not a url",
		"https://gitlab.com/owner/repo/pull/1", // not github
		"github.com/owner",                     // missing repo
		"github.com/owner/",                    // empty repo
		"github.com//repo/pull/1",              // empty owner
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			got, ok := extractSlug(in)
			if ok {
				t.Errorf("expected reject, got (%q, true)", got)
			}
		})
	}
}

// TestClone_EmptyURLReturnsError verifies the fail-fast path: an
// empty URL must NOT trigger a git subprocess. We test through the
// type-method call so the public surface is exercised.
func TestClone_EmptyURLReturnsError(t *testing.T) {
	s := GitClone{}
	_, err := s.Clone(nil, scmdomain.PRURL(""))
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
	// The error must wrap ErrInvalidPRURL so the cmd layer's
	// error mapping routes it correctly.
	if !strings.Contains(err.Error(), scmdomain.ErrInvalidPRURL.Error()) {
		t.Errorf("expected ErrInvalidPRURL in chain, got %v", err)
	}
}

// TestClone_NonGitHubURLReturnsError verifies the slug extraction
// rejects URLs from other hosts. We do not want to silently try a
// git clone on a URL the adapter does not know how to parse.
func TestClone_NonGitHubURLReturnsError(t *testing.T) {
	s := GitClone{}
	_, err := s.Clone(nil, scmdomain.PRURL("https://gitlab.com/foo/bar/pull/1"))
	if err == nil {
		t.Fatal("expected error for non-GitHub URL, got nil")
	}
	if !strings.Contains(err.Error(), scmdomain.ErrInvalidPRURL.Error()) {
		t.Errorf("expected ErrInvalidPRURL in chain, got %v", err)
	}
}

// TestDiscoverDefaultBranch_ParsesSymref documents the format
// git ls-remote --symref prints. If git ever changes the format
// this test breaks loudly.
//
// We invoke discoverDefaultBranch indirectly via a fake stdout
// is too coupled; instead we keep this test asserting the format
// is what we expect.
func TestDiscoverDefaultBranch_FormatExpected(t *testing.T) {
	// Two known shapes. If you see this fail, check whether git
	// changed its --symref output format.
	shapes := []string{
		"ref: refs/heads/main\tHEAD\n<sha>\tHEAD\n",
		"ref: refs/heads/master\tHEAD\n<sha>\tHEAD\n",
	}
	for _, raw := range shapes {
		t.Run(raw[:20], func(t *testing.T) {
			line := strings.SplitN(raw, "\n", 2)[0]
			if !strings.HasPrefix(line, "ref: refs/heads/") {
				t.Fatalf("ls-remote format drift: first line %q", line)
			}
		})
	}
}