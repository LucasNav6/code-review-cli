package claudereview_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasNav6/code-review-cli/internal/claudereview"
)

// standardColours is the minimal palette used by every test in this
// file. We do not assert exact ANSI sequences (those depend on lipgloss
// internals) — we only assert that the rendered bytes contain the
// expected human-readable substrings.
var standardColours = claudereview.Colours{
	Foreground: "#FAFAFA",
	Muted:      "#8B949E",
	Added:      "#3FB950",
	Removed:    "#F85149",
	Category:   "#F59E0B",
	Accent:     "#58A6FF",
}

// TestRenderer_RendersCommonFields is the smoke test: every category
// should at least emit the title, the file:line reference, and the
// suggestion block. If any of those disappears for a category, the
// renderer regressed.
func TestRenderer_RendersCommonFields(t *testing.T) {
	cases := []struct {
		name     string
		category string
	}{
		{"resilience", claudereview.CategoryResilience},
		{"readability", claudereview.CategoryReadability},
		{"security", claudereview.CategorySecurity},
		{"testing", claudereview.CategoryTesting},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requires := true
			f := claudereview.Finding{
				Title:         "titulo de prueba",
				Context:       "contexto de prueba",
				Impact:        []string{"impacto uno", "impacto dos"},
				Suggestion:    "sugerencia de prueba",
				Category:      tc.category,
				File:          "internal/foo/bar.go",
				Line:          42,
				Snippets:      []claudereview.Snippet{{Line: 42, Code: "+ foo := 1"}},
				RequiresTests: &requires,
			}

			r := claudereview.NewRenderer(standardColours)
			var buf bytes.Buffer
			r.Render(&buf, f)

			out := buf.String()
			for _, want := range []string{
				"titulo de prueba",
				"internal/foo/bar.go:42",
				"sugerencia de prueba",
				"Impact",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("rendered output missing %q\n--- output ---\n%s", want, out)
				}
			}
		})
	}
}

// TestRenderer_SecurityShowsOWASP verifies the SECURITY-specific
// block ("OWASP" header + value) appears only when the finding is a
// SECURITY finding with a non-empty OWASP field.
func TestRenderer_SecurityShowsOWASP(t *testing.T) {
	f := claudereview.Finding{
		Title:      "Falta autorizacion",
		Category:   claudereview.CategorySecurity,
		OWASP:      "API1:2023",
		File:       "x.go",
		Line:       1,
		Suggestion: "validar",
	}

	r := claudereview.NewRenderer(standardColours)
	var buf bytes.Buffer
	r.Render(&buf, f)

	out := buf.String()
	if !strings.Contains(out, "OWASP") {
		t.Errorf("expected OWASP header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "API1:2023") {
		t.Errorf("expected API1:2023 value in output, got:\n%s", out)
	}
}

// TestRenderer_NonSecurityDoesNotShowOWASP ensures the OWASP block
// does not leak into other categories even if the OWASP field is
// accidentally populated.
func TestRenderer_NonSecurityDoesNotShowOWASP(t *testing.T) {
	cases := []string{
		claudereview.CategoryResilience,
		claudereview.CategoryReadability,
		claudereview.CategoryTesting,
	}
	for _, cat := range cases {
		t.Run(cat, func(t *testing.T) {
			f := claudereview.Finding{
				Title:    "x",
				Category: cat,
				OWASP:    "API1:2023", // stray value
				File:     "x.go",
				Line:     1,
			}
			r := claudereview.NewRenderer(standardColours)
			var buf bytes.Buffer
			r.Render(&buf, f)

			if strings.Contains(buf.String(), "OWASP") {
				t.Errorf("OWASP block leaked into %s rendering", cat)
			}
		})
	}
}

// TestRenderer_TestingShowsTestingBlock verifies that a TESTING
// finding with all four testing-specific fields renders each of them
// under a "Testing" header.
func TestRenderer_TestingShowsTestingBlock(t *testing.T) {
	requires := true
	f := claudereview.Finding{
		Title:         "Falta cobertura",
		Category:      claudereview.CategoryTesting,
		File:          "x.go",
		Line:          10,
		RequiresTests: &requires,
		TestsCovered:  "Solo el caso feliz.",
		TestsMissing:  "Lista vacia y error.",
		EdgeCase:      "Input vacio.",
	}

	r := claudereview.NewRenderer(standardColours)
	var buf bytes.Buffer
	r.Render(&buf, f)

	out := buf.String()
	for _, want := range []string{
		"Testing",
		"Requires tests: yes",
		"Tests covered:",
		"Tests missing:",
		"Edge case:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestRenderer_TestingRequiresTestsFalse verifies the "no" branch of
// the requires-tests toggle. This matters because we use *bool so
// nil is also a valid value and we want to make sure nil, true, and
// false all render correctly.
func TestRenderer_TestingRequiresTestsFalse(t *testing.T) {
	requires := false
	f := claudereview.Finding{
		Title:         "x",
		Category:      claudereview.CategoryTesting,
		File:          "x.go",
		Line:          1,
		RequiresTests: &requires,
	}

	r := claudereview.NewRenderer(standardColours)
	var buf bytes.Buffer
	r.Render(&buf, f)

	if !strings.Contains(buf.String(), "Requires tests: no") {
		t.Errorf("expected 'Requires tests: no', got:\n%s", buf.String())
	}
}

// TestRenderer_NonTestingDoesNotShowTestingBlock ensures the TESTING
// block does not leak into other categories even if the testing
// fields are accidentally populated.
func TestRenderer_NonTestingDoesNotShowTestingBlock(t *testing.T) {
	requires := true
	cases := []string{
		claudereview.CategoryResilience,
		claudereview.CategoryReadability,
		claudereview.CategorySecurity,
	}
	for _, cat := range cases {
		t.Run(cat, func(t *testing.T) {
			f := claudereview.Finding{
				Title:         "x",
				Category:      cat,
				File:          "x.go",
				Line:          1,
				RequiresTests: &requires,
				TestsMissing:  "noise",
			}
			r := claudereview.NewRenderer(standardColours)
			var buf bytes.Buffer
			r.Render(&buf, f)

			if strings.Contains(buf.String(), "Testing") {
				t.Errorf("Testing block leaked into %s rendering", cat)
			}
		})
	}
}

// TestRenderer_RenderNoFindingsRendersAlways verifies the
// no-findings card still renders even when called for any category
// (the caller picks the wording, the renderer just paints the box).
func TestRenderer_RenderNoFindingsRendersAlways(t *testing.T) {
	r := claudereview.NewRenderer(standardColours)
	var buf bytes.Buffer
	r.RenderNoFindings(&buf)

	out := buf.String()
	if !strings.Contains(out, "No review findings") {
		t.Errorf("expected success message, got:\n%s", out)
	}
}
