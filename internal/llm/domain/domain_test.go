package domain_test

import (
	"errors"
	"testing"

	"github.com/LucasNav6/code-review-cli/internal/llm/domain"
)

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