package claudereview_test

import (
	"strings"
	"testing"

	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/claudereview"
)

// TestParse_Empty verifies the documented error when the response is
// the empty string. This is the most degenerate failure mode and is
// the one callers are most likely to hit on misconfigured runs.
func TestParse_Empty(t *testing.T) {
	got, err := claudereview.Parse("")
	if got != nil {
		t.Fatalf("expected nil findings, got %d", len(got))
	}
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), helpers.ErrEmptyResponse.Error()) {
		t.Fatalf("expected error to wrap ErrEmptyResponse, got %v", err)
	}
}

// TestParse_NoFindingsLiteral verifies the literal "NO_FINDINGS"
// shortcut the prompts explicitly allow. We accept it both bare and
// inside markdown fences for tolerance.
func TestParse_NoFindingsLiteral(t *testing.T) {
	cases := []string{
		"NO_FINDINGS",
		"  NO_FINDINGS  ",
		"```json\nNO_FINDINGS\n```",
		"```\nNO_FINDINGS\n```",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			got, err := claudereview.Parse(in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("expected non-nil empty slice")
			}
			if len(got) != 0 {
				t.Fatalf("expected empty findings, got %d", len(got))
			}
		})
	}
}

// TestParse_EmptyJSONArray verifies the JSON shape `{"findings": []}`
// parses without error and yields an empty slice.
func TestParse_EmptyJSONArray(t *testing.T) {
	got, err := claudereview.Parse(`{"findings": []}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty findings, got %#v", got)
	}
}

// TestParse_MalformedJSON verifies non-JSON input is rejected with
// ErrMalformedResponse (or a wrapping of helpers.ErrMalformedResponse).
func TestParse_MalformedJSON(t *testing.T) {
	cases := []string{
		"this is not json",
		`{"findings": "not an array"}`,
		`{`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			got, err := claudereview.Parse(in)
			if got != nil {
				t.Fatalf("expected nil findings, got %d", len(got))
			}
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), helpers.ErrMalformedResponse.Error()) {
				t.Fatalf("expected error to wrap ErrMalformedResponse, got %v", err)
			}
		})
	}
}

// TestParse_Resilience verifies the canonical RESILIENCE payload —
// the one that was supported before the multi-category refactor.
// We make sure none of the category-specific optional fields leak in
// or change the parsed shape.
func TestParse_Resilience(t *testing.T) {
	input := `{
		"findings": [{
			"title": "Falta manejar fallo del iframe",
			"context": "El handler ignora el error.",
			"impact": ["El usuario ve una pantalla en blanco."],
			"suggestion": "Sumar un fallback con mensaje accionable.",
			"category": "RESILIENCE",
			"file": "internal/foo/bar.go",
			"line": 42,
			"snippets": [{"line": 42, "code": "+ iframe.onerror = () => null"}]
		}]
	}`

	got, err := claudereview.Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}

	f := got[0]
	if f.Title != "Falta manejar fallo del iframe" {
		t.Errorf("title: got %q", f.Title)
	}
	if f.Category != claudereview.CategoryResilience {
		t.Errorf("category: got %q, want RESILIENCE", f.Category)
	}
	if f.OWASP != "" {
		t.Errorf("OWASP must be empty for non-security findings, got %q", f.OWASP)
	}
	if f.RequiresTests != nil {
		t.Errorf("RequiresTests must be nil for non-testing findings, got %v", *f.RequiresTests)
	}
}

// TestParse_Readability covers the maintainability prompt's shape.
// No category-specific fields; only common ones.
func TestParse_Readability(t *testing.T) {
	input := `{
		"findings": [{
			"title": "Numero magico sin constante",
			"context": "Se usa 86400 sin explicar que representa.",
			"impact": ["Dificil de entender para nuevos mantenedores."],
			"suggestion": "Reemplazar por SECONDS_PER_DAY.",
			"category": "READABILITY",
			"file": "internal/foo/bar.go",
			"line": 12,
			"snippets": [{"line": 12, "code": "+ if t.Unix() == 86400 {"}]
		}]
	}`

	got, err := claudereview.Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(got))
	}
	if got[0].Category != claudereview.CategoryReadability {
		t.Errorf("category: got %q", got[0].Category)
	}
	if got[0].OWASP != "" {
		t.Errorf("OWASP must be empty for readability findings, got %q", got[0].OWASP)
	}
}

// TestParse_Security verifies the SECURITY payload preserves the
// OWASP field. Also covers the default-OWASP behaviour when the LLM
// omits the field on a security finding.
func TestParse_Security(t *testing.T) {
	t.Run("with owasp", func(t *testing.T) {
		input := `{
			"findings": [{
				"title": "Falta chequeo de autorizacion",
				"context": "El endpoint devuelve el recurso por id sin validar.",
				"impact": ["Acceso a datos de otros usuarios."],
				"suggestion": "Validar session.userId == resource.ownerId.",
				"category": "SECURITY",
				"owasp": "API1:2023",
				"file": "internal/api/users.go",
				"line": 87,
				"snippets": [{"line": 87, "code": "+ resource := db.GetByID(userID)"}]
			}]
		}`

		got, err := claudereview.Parse(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 finding")
		}
		if got[0].Category != claudereview.CategorySecurity {
			t.Errorf("category: got %q", got[0].Category)
		}
		if got[0].OWASP != "API1:2023" {
			t.Errorf("OWASP: got %q", got[0].OWASP)
		}
	})

	t.Run("missing owasp is backfilled with sentinel", func(t *testing.T) {
		input := `{
			"findings": [{
				"title": "Algo de seguridad",
				"context": "...",
				"impact": ["..."],
				"suggestion": "...",
				"category": "SECURITY",
				"file": "x.go",
				"line": 1
			}]
		}`

		got, err := claudereview.Parse(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[0].OWASP != "API0:2023" {
			t.Errorf("expected backfilled OWASP sentinel API0:2023, got %q", got[0].OWASP)
		}
	})
}

// TestParse_Testing verifies the TESTING payload preserves all four
// testing-specific optional fields, including RequiresTests as a
// *bool so we can distinguish "true" from "absent".
func TestParse_Testing(t *testing.T) {
	input := `{
		"findings": [{
			"title": "Falta cobertura para lista vacia",
			"context": "Solo se cubre el camino feliz.",
			"impact": ["Una regresion en el edge case pasaria inadvertida."],
			"suggestion": "Sumar un test con input vacio.",
			"category": "TESTING",
			"requires_tests": true,
			"tests_covered": "Solo el caso positivo.",
			"tests_missing": "Lista vacia y error de la dependencia.",
			"edge_case": "Input de longitud 0.",
			"file": "internal/foo/list.go",
			"line": 23,
			"snippets": [{"line": 23, "code": "+ if len(items) == 0 {"}]
		}]
	}`

	got, err := claudereview.Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 finding")
	}

	f := got[0]
	if f.Category != claudereview.CategoryTesting {
		t.Errorf("category: got %q", f.Category)
	}
	if f.RequiresTests == nil {
		t.Fatal("RequiresTests must be non-nil for testing findings")
	}
	if !*f.RequiresTests {
		t.Errorf("RequiresTests: got false, want true")
	}
	if f.TestsCovered != "Solo el caso positivo." {
		t.Errorf("TestsCovered: got %q", f.TestsCovered)
	}
	if f.TestsMissing != "Lista vacia y error de la dependencia." {
		t.Errorf("TestsMissing: got %q", f.TestsMissing)
	}
	if f.EdgeCase != "Input de longitud 0." {
		t.Errorf("EdgeCase: got %q", f.EdgeCase)
	}
}

// TestParse_MalformedFinding covers the validator that rejects
// findings missing the required `file` or `title` fields. Those two
// are the bare minimum needed to render anything useful.
func TestParse_MalformedFinding(t *testing.T) {
	cases := map[string]string{
		"missing title": `{"findings":[{"file":"x.go","line":1}]}`,
		"missing file":  `{"findings":[{"title":"x","line":1}]}`,
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := claudereview.Parse(in)
			if got != nil {
				t.Fatalf("expected nil findings, got %d", len(got))
			}
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), helpers.ErrMalformedFinding.Error()) {
				t.Fatalf("expected error to wrap ErrMalformedFinding, got %v", err)
			}
		})
	}
}

// TestParse_CodeFencesAreStripped confirms the parser strips markdown
// fences around valid JSON without changing the parsed payload. This
// is a defensive measure because the LLM occasionally adds them
// despite explicit instructions not to.
func TestParse_CodeFencesAreStripped(t *testing.T) {
	input := "```json\n{\"findings\":[]}\n```"
	got, err := claudereview.Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty findings, got %d", len(got))
	}
}

// TestParse_UnknownCategoryFallsBackToResilience verifies that if the
// LLM emits a typo'd category, the parser coerces it to RESILIENCE
// (the documented default) rather than passing an unknown string to
// the renderer.
func TestParse_UnknownCategoryFallsBackToResilience(t *testing.T) {
	input := `{
		"findings": [{
			"title": "Algo",
			"context": "...",
			"impact": ["..."],
			"suggestion": "...",
			"category": "RESILENCE",
			"file": "x.go",
			"line": 1
		}]
	}`

	got, err := claudereview.Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Category != claudereview.CategoryResilience {
		t.Errorf("expected fallback to RESILIENCE, got %q", got[0].Category)
	}
}

// TestParse_LowercaseCategoryUppercased verifies the category is
// canonicalised so renderers can switch on it without worrying about
// the casing the LLM happened to produce.
func TestParse_LowercaseCategoryUppercased(t *testing.T) {
	input := `{
		"findings": [{
			"title": "Algo",
			"context": "...",
			"impact": ["..."],
			"suggestion": "...",
			"category": "security",
			"owasp": "API4:2023",
			"file": "x.go",
			"line": 1
		}]
	}`

	got, err := claudereview.Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Category != claudereview.CategorySecurity {
		t.Errorf("expected uppercased category, got %q", got[0].Category)
	}
}
