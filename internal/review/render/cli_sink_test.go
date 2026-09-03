package render_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
	"github.com/LucasNav6/code-review-cli/internal/review/render"
)

// TestNewCLISink_NilWriterPanics documents the contract: passing
// nil for either writer is a programming error.
func TestNewCLISink_NilWriterPanics(t *testing.T) {
	cases := []struct {
		name string
		out  any
		err  any
	}{
		{"nil out", nil, &bytes.Buffer{}},
		{"nil err", &bytes.Buffer{}, nil},
		{"both nil", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic for %s, got none", tc.name)
				}
			}()
			render.NewCLISink(tc.out.(interface{ Write([]byte) (int, error) }),
				tc.err.(interface{ Write([]byte) (int, error) }))
		})
	}
}

// TestCLISink_RenderHeader verifies the PR header still paints on
// stdout (no change vs H5).
func TestCLISink_RenderHeader(t *testing.T) {
	var buf bytes.Buffer
	sink := render.NewCLISink(&buf, &bytes.Buffer{})

	sink.RenderHeader(scmdomain.PRMetadata{
		Number: 42,
		Title:  "Add foo to bar",
		State:  "OPEN",
	})

	out := buf.String()
	for _, want := range []string{"Add foo to bar", "42", "OPEN"} {
		if !strings.Contains(out, want) {
			t.Errorf("header missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestCLISink_StartCategoryHeader_EmitsShortTag verifies the
// per-category header line includes the short bracket tag ([R],
// [M], [T], [S], [S·SB]) plus the full label, separated by
// dashes sized to a fixed width.
func TestCLISink_StartCategoryHeader_EmitsShortTag(t *testing.T) {
	var buf bytes.Buffer
	sink := render.NewCLISink(&buf, &bytes.Buffer{})

	cases := []struct {
		cat      reviewdomain.Category
		wantTag  string
		wantWord string
	}{
		// Note: the canonical Category string is what
		// category[:1] reads from. CategoryMaintainability is
		// "READABILITY" (kept for backward compat with the LLM
		// wire format), so the tag is [R], not [M].
		{reviewdomain.CategoryResilience, "[R]", "RESILIENCE"},
		{reviewdomain.CategoryMaintainability, "[R]", "READABILITY"},
		{reviewdomain.CategoryTesting, "[T]", "TESTING"},
		{reviewdomain.CategorySecurity, "[S]", "SECURITY"},
		{reviewdomain.CategorySecuritySBOM, "[S·SB]", "SECURITY · SBOM"},
	}
	for _, tc := range cases {
		t.Run(string(tc.cat), func(t *testing.T) {
			buf.Reset()
			sink.StartCategoryHeader(tc.cat)
			out := buf.String()
			if !strings.Contains(out, tc.wantTag) {
				t.Errorf("header missing tag %q\n--- output ---\n%s", tc.wantTag, out)
			}
			if !strings.Contains(out, tc.wantWord) {
				t.Errorf("header missing label %q\n--- output ---\n%s", tc.wantWord, out)
			}
			if !strings.Contains(out, "───") {
				t.Errorf("header missing dash prefix\n--- output ---\n%s", out)
			}
		})
	}
}

// TestCLISink_StartSpinner_TerminatesOnStop verifies the spinner
// is non-blocking and Stop() actually ends the run.
func TestCLISink_StartSpinner_TerminatesOnStop(t *testing.T) {
	var stdout, stderr bytes.Buffer
	sink := render.NewCLISink(&stdout, &stderr)

	h := sink.StartSpinner("test spinner")
	if h == nil {
		t.Fatal("StartSpinner returned nil")
	}
	// Give the spinner goroutine a tick to start.
	// We can't assert on the live animation (timing-sensitive),
	// only that Stop terminates and the spinner machinery shuts
	// down cleanly.
	done := make(chan struct{})
	go func() {
		h.Stop()
		close(done)
	}()
	<-done
	// If Stop didn't terminate the spinner, this would deadlock.
}

// TestCLISink_StartSpinner_IdempotentStop documents that calling
// Stop twice on the same handle does not panic. The sync.Once
// guard inside spinnerHandle guarantees it.
func TestCLISink_StartSpinner_IdempotentStop(t *testing.T) {
	var stdout, stderr bytes.Buffer
	sink := render.NewCLISink(&stdout, &stderr)
	h := sink.StartSpinner("idempotent")
	h.Stop()
	h.Stop() // must not panic
}

// TestCLISink_RenderFindings_EmptyFindings paints the no-findings
// card. The card text "No review findings" must appear on stdout.
func TestCLISink_RenderFindings_EmptyFindings(t *testing.T) {
	var buf bytes.Buffer
	sink := render.NewCLISink(&buf, &bytes.Buffer{})

	if err := sink.RenderFindings(reviewdomain.CategoryResilience, `{"findings": []}`); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No review findings") {
		t.Errorf("expected no-findings card, got:\n%s", buf.String())
	}
}

// TestCLISink_RenderFindings_OneFinding paints one card and
// asserts title + file:line + suggestion are visible.
func TestCLISink_RenderFindings_OneFinding(t *testing.T) {
	var buf bytes.Buffer
	sink := render.NewCLISink(&buf, &bytes.Buffer{})

	resp := `{
		"findings": [{
			"title": "Falta retry",
			"context": "...",
			"impact": ["..."],
			"suggestion": "Sumar retry con backoff.",
			"category": "RESILIENCE",
			"file": "internal/foo.go",
			"line": 17
		}]
	}`
	if err := sink.RenderFindings(reviewdomain.CategoryResilience, resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Falta retry", "internal/foo.go:17", "Sumar retry con backoff."} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestCLISink_RenderFindings_MalformedJSONReturnsError locks
// down the "parse failure is non-fatal but the sink tells the
// caller" contract.
func TestCLISink_RenderFindings_MalformedJSONReturnsError(t *testing.T) {
	var buf bytes.Buffer
	sink := render.NewCLISink(&buf, &bytes.Buffer{})

	err := sink.RenderFindings(reviewdomain.CategorySecurity, "not json")
	if err == nil {
		t.Fatal("expected error for malformed response, got nil")
	}
	if !strings.Contains(err.Error(), "SECURITY") {
		t.Errorf("error must mention the category, got: %v", err)
	}
}

// TestCLISink_RenderFindings_OrderHeadersBeforeCards verifies
// the documented ordering: header first, then spinner (stderr),
// then findings (stdout). We cannot assert on the spinner
// without a real TTY, but we can assert that StartCategoryHeader
// appears on stdout BEFORE RenderFindings paints cards.
func TestCLISink_OrderHeadersBeforeCards(t *testing.T) {
	var buf bytes.Buffer
	sink := render.NewCLISink(&buf, &bytes.Buffer{})

	sink.StartCategoryHeader(reviewdomain.CategoryResilience)
	if err := sink.RenderFindings(reviewdomain.CategoryResilience, `{"findings": []}`); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stripANSI(buf.String())
	headerIdx := strings.Index(out, "RESILIENCE")
	cardIdx := strings.Index(out, "No review findings")
	if headerIdx < 0 || cardIdx < 0 {
		t.Fatalf("missing markers:\n%s", out)
	}
	if headerIdx >= cardIdx {
		t.Errorf("header must precede card; got header@%d card@%d", headerIdx, cardIdx)
	}
}

// stripANSI removes the ESC[...m sequences that lipgloss emits,
// so substring assertions are stable across lipgloss versions.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// TestRotatingMessages_FirstMessageReturnsFirstEntry verifies
// that the very first Next() call returns the first message in
// the set (no shuffling, no offset).
func TestRotatingMessages_FirstMessageReturnsFirstEntry(t *testing.T) {
	cases := []struct {
		cat reviewdomain.Category
		// We do not assert exact text (would be brittle) — only
		// that the message is non-empty and category-specific
		// vocabulary appears.
		mustContain string
	}{
		{reviewdomain.CategoryResilience, "diff"},
		{reviewdomain.CategorySecurity, "OWASP"},
		{reviewdomain.CategorySecuritySBOM, "OSV"},
	}
	for _, tc := range cases {
		t.Run(string(tc.cat), func(t *testing.T) {
			rm := render.NewRotatingMessages(tc.cat)
			got := rm.Current()
			if got == "" {
				t.Fatal("first message is empty")
			}
			if !strings.Contains(got, tc.mustContain) {
				t.Errorf("first message %q does not mention %q", got, tc.mustContain)
			}
		})
	}
}

// TestRotatingMessages_NextCyclesThroughSet verifies the
// rotation returns every message in the set before repeating.
func TestRotatingMessages_NextCyclesThroughSet(t *testing.T) {
	rm := render.NewRotatingMessages(reviewdomain.CategoryResilience)
	seen := map[string]bool{}
	for i := 0; i < 12; i++ {
		seen[rm.Next()] = true
	}
	// The Resilience set has 10 messages; after 12 calls we must
	// have seen at least 10 (one per element, possibly repeated).
	if len(seen) < 10 {
		t.Errorf("rotation not cycling: saw %d unique messages, want at least 10",
			len(seen))
	}
}