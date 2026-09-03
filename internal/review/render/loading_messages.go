package render

import (
	"sync/atomic"
	"time"

	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
)

// RotatingMessages holds the rotating spinner messages for one
// review invocation. Each category has a small set of canned
// messages; every rotationInterval the next message in the list
// is surfaced via Next().
//
// We use a counter (not a clock) because:
//   - tests do not need to wait for actual rotation,
//   - the CLI invocation is short (a few seconds), so messages
//     may not actually rotate more than once — that's fine,
//   - it keeps the API simple (no Start/Stop goroutine needed).
//
// The next-call counter is thread-safe so the spinner goroutine
// (in CLISink.StartSpinner) and the test that wants to assert the
// first message both work.
type RotatingMessages struct {
	messages []string
	index    uint64 // atomic counter, last used index +1
}

// NewRotatingMessages returns the rotating-message set for the
// given category. Unknown categories get a generic default set
// so the spinner is never empty.
//
// The set is intentionally small (10 messages per category) —
// the user only sees one at a time and a long review rarely
// cycles through more than 2-3 of them.
func NewRotatingMessages(cat reviewdomain.Category) *RotatingMessages {
	msgs := messagesFor(cat)
	return &RotatingMessages{messages: msgs}
}

// Next returns the next message in the rotation. Safe to call
// from any goroutine.
func (r *RotatingMessages) Next() string {
	if len(r.messages) == 0 {
		return ""
	}
	i := atomic.AddUint64(&r.index, 1) - 1
	return r.messages[int(i%uint64(len(r.messages)))]
}

// Current returns the message that was last returned by Next
// without advancing the counter. Used by tests that want to
// assert "the first message is X".
func (r *RotatingMessages) Current() string {
	if len(r.messages) == 0 {
		return ""
	}
	i := atomic.LoadUint64(&r.index)
	if i == 0 {
		return r.messages[0]
	}
	return r.messages[(i-1)%uint64(len(r.messages))]
}

// RotationInterval is how often the CLI swaps the spinner
// message. We picked 20s because that matches the user-facing
// contract ("the text changes every 20s"); the use case does
// not know about this constant — the spinner goroutine in
// CLISink owns the ticker.
const RotationInterval = 20 * time.Second

// messagesFor returns the canned spinner messages for a category.
// Each category has ~10 entries; rotation is per category so the
// user always sees relevant vocabulary (e.g. RESILIENCE does not
// show "checking OWASP API Top 10").
func messagesFor(cat reviewdomain.Category) []string {
	switch cat {
	case reviewdomain.CategoryResilience:
		return []string{
			"reading the diff for missing error handling",
			"checking retry / backoff patterns",
			"scanning for unhandled edge cases",
			"looking for timeout / cancellation gaps",
			"checking observability signals (logs, metrics, traces)",
			"verifying graceful degradation paths",
			"inspecting fallback behaviour on external failures",
			"checking transaction / partial-failure boundaries",
			"reading for silent error swallowing",
			"auditing crash-and-restart recovery",
		}
	case reviewdomain.CategoryMaintainability:
		return []string{
			"reading the diff for code clarity",
			"checking naming consistency",
			"scanning for duplicated logic",
			"looking for overcomplicated conditionals",
			"checking magic numbers / strings",
			"verifying function size and responsibilities",
			"inspecting comment quality",
			"checking dead code / unused exports",
			"reading for hidden coupling",
			"auditing test coverage hints",
		}
	case reviewdomain.CategoryTesting:
		return []string{
			"reading the diff for missing test cases",
			"checking edge case coverage",
			"scanning for untested error paths",
			"looking for missing mocks / fixtures",
			"checking integration vs unit balance",
			"verifying assertions are meaningful",
			"inspecting flaky-test risks (time, randomness)",
			"checking test naming and clarity",
			"reading for over-mocked units",
			"auditing skipped tests",
		}
	case reviewdomain.CategorySecurity:
		return []string{
			"reading the diff against OWASP API Top 10",
			"checking authorization (BOLA / BFLA)",
			"scanning for authentication weaknesses",
			"looking for SSRF risks on user-controlled URLs",
			"checking input validation boundaries",
			"verifying sensitive data handling",
			"inspecting secrets / PII exposure",
			"checking security headers and CORS",
			"reading for insecure deserialization",
			"auditing rate limiting and abuse prevention",
		}
	case reviewdomain.CategorySecuritySBOM:
		return []string{
			"reading the SBOM against OSV.dev",
			"checking components against CVE feed",
			"scanning for vulnerable transitive dependencies",
			"looking for outdated packages with known fixes",
			"checking CVSS severity of open CVEs",
			"verifying fixed versions are documented",
			"inspecting package provenance and licenses",
			"checking for deprecated / unmaintained deps",
			"reading for typosquatted package names",
			"auditing lockfile pinning",
		}
	}
	// Unknown category: a safe default that does not commit to
	// any specific check. The use case should never pass an
	// unknown category here, but if it does (refactor drift),
	// we degrade gracefully.
	return []string{
		"reading the diff",
		"analyzing the changes",
		"checking for issues",
	}
}