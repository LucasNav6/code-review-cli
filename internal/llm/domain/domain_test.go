package domain_test

import (
	"errors"
	"testing"

	"github.com/LucasNav6/code-review-cli/internal/llm/domain"
)

// TestNewResponse_TrimsWhitespace documents the constructor's
// whitespace semantics. Adapters do not need to trim — the wrapper
// guarantees it.
func TestNewResponse_TrimsWhitespace(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"  hello  ", "hello"},
		{"\nhello\n", "hello"},
		{"\t hello \r\n", "hello"},
		{"   ", ""},
		{"", ""},
		{"hello world  ", "hello world"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			r := domain.NewResponse(tc.in)
			if r.Body != tc.want {
				t.Errorf("NewResponse(%q).Body = %q, want %q",
					tc.in, r.Body, tc.want)
			}
		})
	}
}

// TestSentinelsAreDistinct makes sure each sentinel can be matched
// with errors.Is independently. If two of them were the same value
// the cmd layer would misroute errors.
func TestSentinelsAreDistinct(t *testing.T) {
	sentinels := []error{
		domain.ErrProviderNotImpl,
		domain.ErrUnknownProvider,
		domain.ErrProviderInvalid,
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("sentinel %d (%v) matches sentinel %d (%v) — they must be distinct",
					i, a, j, b)
			}
		}
	}
}