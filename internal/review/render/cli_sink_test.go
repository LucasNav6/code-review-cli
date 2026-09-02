package render_test

import (
	"bytes"
	"strings"
	"testing"

	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
	"github.com/LucasNav6/code-review-cli/internal/review/render"
)

// TestNewCLISink_NilWriterPanics documents the contract: passing
// nil is a programming error. We panic rather than silently
// producing no output because the user would have no idea why
// nothing printed.
func TestNewCLISink_NilWriterPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil writer, got none")
		}
	}()
	render.NewCLISink(nil)
}

// TestCLISink_RenderHeader_EmitsTitleAndNumber verifies the
// header paints the PR title, the number, and the state. We do
// NOT assert on exact lipgloss output (those depend on lipgloss
// internals); we only assert that the user-visible substrings
// appear.
func TestCLISink_RenderHeader_EmitsTitleAndNumber(t *testing.T) {
	var buf bytes.Buffer
	sink := render.NewCLISink(&buf)

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

// TestCLISink_RenderReviewBlock_EmptyFindings verifies the
// no-findings card renders. The card says "No review findings" —
// we assert the substring is present so we know the renderer took
// the empty path.
func TestCLISink_RenderReviewBlock_EmptyFindings(t *testing.T) {
	var buf bytes.Buffer
	sink := render.NewCLISink(&buf)

	if err := sink.RenderReviewBlock(reviewdomain.CategoryResilience, `{"findings": []}`); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No review findings") {
		t.Errorf("expected no-findings card, got:\n%s", buf.String())
	}
}

// TestCLISink_RenderReviewBlock_OneFinding paints one finding and
// asserts that title and file:line show up. We do not assert on
// ANSI sequences (lipgloss v2 internals).
func TestCLISink_RenderReviewBlock_OneFinding(t *testing.T) {
	var buf bytes.Buffer
	sink := render.NewCLISink(&buf)

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
	if err := sink.RenderReviewBlock(reviewdomain.CategoryResilience, resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Falta retry",
		"internal/foo.go:17",
		"Sumar retry con backoff.",
		"RESILIENCE",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestCLISink_RenderReviewBlock_MalformedResponseReturnsError
// locks down the "parse failure is non-fatal but the sink tells
// the caller" contract. The use case matches on this to decide
// whether to warn.
func TestCLISink_RenderReviewBlock_MalformedResponseReturnsError(t *testing.T) {
	var buf bytes.Buffer
	sink := render.NewCLISink(&buf)

	err := sink.RenderReviewBlock(reviewdomain.CategorySecurity, "not json at all")
	if err == nil {
		t.Fatal("expected error for malformed response, got nil")
	}
	// The error message should mention the category so the user
	// knows which pass failed.
	if !strings.Contains(err.Error(), "SECURITY") {
		t.Errorf("error must mention the category, got: %v", err)
	}
}

// TestCLISink_RenderReviewBlock_AllCategories paints one finding
// for each of the four categories to verify the renderer does not
// leak category-specific fields (e.g. OWASP block into RESILIENCE).
func TestCLISink_RenderReviewBlock_AllCategories(t *testing.T) {
	cases := []struct {
		category reviewdomain.Category
		resp     string
		mustHave []string
		mustNot  []string
	}{
		{
			category: reviewdomain.CategoryResilience,
			resp: `{"findings":[{
				"title":"x","context":"c","impact":["i"],
				"suggestion":"s","category":"RESILIENCE",
				"file":"x.go","line":1
			}]}`,
			mustHave: []string{"x", "x.go:1"},
			mustNot:  []string{"OWASP", "Requires tests"},
		},
		{
			category: reviewdomain.CategorySecurity,
			resp: `{"findings":[{
				"title":"y","context":"c","impact":["i"],
				"suggestion":"s","category":"SECURITY",
				"owasp":"API1:2023",
				"file":"y.go","line":2
			}]}`,
			mustHave: []string{"y", "y.go:2", "API1:2023", "OWASP"},
			mustNot:  []string{"Requires tests"},
		},
		{
			category: reviewdomain.CategoryTesting,
			resp: `{"findings":[{
				"title":"z","context":"c","impact":["i"],
				"suggestion":"s","category":"TESTING",
				"requires_tests":true,
				"tests_missing":"missing",
				"file":"z.go","line":3
			}]}`,
			mustHave: []string{"z", "z.go:3", "Requires tests", "Tests missing"},
			mustNot:  []string{"OWASP"},
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.category), func(t *testing.T) {
			var buf bytes.Buffer
			sink := render.NewCLISink(&buf)

			if err := sink.RenderReviewBlock(tc.category, tc.resp); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			out := buf.String()
			for _, want := range tc.mustHave {
				if !strings.Contains(out, want) {
					t.Errorf("%s output missing %q\n--- output ---\n%s",
						tc.category, want, out)
				}
			}
			for _, unwanted := range tc.mustNot {
				if strings.Contains(out, unwanted) {
					t.Errorf("%s output leaked %q\n--- output ---\n%s",
						tc.category, unwanted, out)
				}
			}
		})
	}
}

// TestCLISink_HeaderThenBlocks verifies the order: header first,
// then blocks. The cmd layer relies on this — the user reads
// top-to-bottom.
func TestCLISink_HeaderThenBlocks(t *testing.T) {
	var buf bytes.Buffer
	sink := render.NewCLISink(&buf)

	sink.RenderHeader(scmdomain.PRMetadata{Number: 1, Title: "PR", State: "OPEN"})
	if err := sink.RenderReviewBlock(reviewdomain.CategoryResilience, `{"findings":[]}`); err != nil {
		t.Fatalf("block: %v", err)
	}

	out := buf.String()
	headerIdx := strings.Index(out, "PR")
	blockIdx := strings.Index(out, "No review findings")
	if headerIdx < 0 || blockIdx < 0 {
		t.Fatalf("missing markers in output:\n%s", out)
	}
	if headerIdx >= blockIdx {
		t.Errorf("header must come before blocks; got header@%d block@%d",
			headerIdx, blockIdx)
	}
}