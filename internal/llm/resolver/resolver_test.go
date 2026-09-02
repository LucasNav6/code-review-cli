package resolver_test

import (
	"errors"
	"testing"

	"github.com/LucasNav6/code-review-cli/internal/llm/adapters/claude"
	"github.com/LucasNav6/code-review-cli/internal/llm/adapters/codex"
	"github.com/LucasNav6/code-review-cli/internal/llm/domain"
	"github.com/LucasNav6/code-review-cli/internal/llm/resolver"
)

// TestStatic_Resolve_KnownNames pins down every name the resolver
// accepts. Adding a new provider means adding a row here.
func TestStatic_Resolve_KnownNames(t *testing.T) {
	r := resolver.New()

	cases := []struct {
		name     string
		wantName string
	}{
		{claude.Name, claude.Name},
		{codex.Name, codex.Name},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := r.Resolve(tc.name)
			if err != nil {
				t.Fatalf("Resolve(%q): unexpected error %v", tc.name, err)
			}
			if p.Name() != tc.wantName {
				t.Errorf("got provider named %q, want %q",
					p.Name(), tc.wantName)
			}
		})
	}
}

// TestStatic_Resolve_EmptyFallsBackToDefault documents the
// documented "empty means default" behaviour. The cmd layer relies
// on this when --provider was not passed.
func TestStatic_Resolve_EmptyFallsBackToDefault(t *testing.T) {
	r := resolver.New()

	p, err := r.Resolve("")
	if err != nil {
		t.Fatalf("Resolve(\"\"): unexpected error %v", err)
	}
	if p.Name() != resolver.Default {
		t.Errorf("got provider named %q, want default %q",
			p.Name(), resolver.Default)
	}
}

// TestStatic_Resolve_UnknownNameErrors locks down the
// ErrUnknownProvider contract — the cmd layer matches on this
// sentinel to render the "known: [claude codex]" hint.
func TestStatic_Resolve_UnknownNameErrors(t *testing.T) {
	r := resolver.New()

	_, err := r.Resolve("openai")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !errors.Is(err, domain.ErrUnknownProvider) {
		t.Fatalf("expected ErrUnknownProvider, got %v", err)
	}
	// The error message must list the known names so the user can
	// see what was actually available.
	msg := err.Error()
	if !contains(msg, "openai") || !contains(msg, "claude") || !contains(msg, "codex") {
		t.Errorf("error message must mention the unknown name and the known ones; got %q", msg)
	}
}

// TestStatic_Resolve_NoWhitespaceTrimming documents that the
// resolver does NOT trim input — a typo with whitespace should fail
// loud so we do not hide user mistakes.
func TestStatic_Resolve_NoWhitespaceTrimming(t *testing.T) {
	r := resolver.New()

	_, err := r.Resolve("  claude  ")
	if err == nil {
		t.Fatal("expected error for whitespace-padded name, got nil")
	}
	if !errors.Is(err, domain.ErrUnknownProvider) {
		t.Fatalf("expected ErrUnknownProvider, got %v", err)
	}
}

// TestStatic_KnownNames_SortedAndStable documents the order of
// KnownNames — the cmd/config command prints this list and we want
// the output to be deterministic.
func TestStatic_KnownNames_SortedAndStable(t *testing.T) {
	r := resolver.New()

	names := r.KnownNames()
	if len(names) == 0 {
		t.Fatal("KnownNames returned empty slice")
	}

	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("KnownNames not sorted: %v", names)
		}
	}

	// Stability: calling it again returns the same order.
	again := r.KnownNames()
	if len(names) != len(again) {
		t.Fatalf("KnownNames length drift: first %d, second %d",
			len(names), len(again))
	}
	for i := range names {
		if names[i] != again[i] {
			t.Fatalf("KnownNames order drift at %d: %q vs %q",
				i, names[i], again[i])
		}
	}
}

// TestStatic_DefaultConstant guards the documented default. The
// cmd layer reads this when the user did not pick a provider.
func TestStatic_DefaultConstant(t *testing.T) {
	if resolver.Default != claude.Name {
		t.Fatalf("Default drift: got %q, want %q (claude)",
			resolver.Default, claude.Name)
	}
}

// contains is a tiny helper that avoids dragging strings.Contains
// into the test file for a single use.
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