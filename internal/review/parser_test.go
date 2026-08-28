package review

import "testing"

func TestParseResult_NoFindingsSentinel(t *testing.T) {
	for _, raw := range []string{"NO_FINDINGS", "no_findings", "  NO FINDINGS  ", "no_finding"} {
		result, err := ParseResult(raw, "SECURITY")
		if err != nil {
			t.Fatalf("ParseResult(%q) returned error: %v", raw, err)
		}

		if len(result.Findings) != 0 {
			t.Fatalf("ParseResult(%q) expected no findings, got %d", raw, len(result.Findings))
		}
	}
}

func TestParseResult_EmptyIsError(t *testing.T) {
	if _, err := ParseResult("   ", "SECURITY"); err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestParseResult_JSON(t *testing.T) {
	raw := `{
		"summary": "algo",
		"findings": [
			{"file": "a.go", "line": 10, "category": "SECURITY", "title": "t", "comment": "c"}
		]
	}`

	result, err := ParseResult(raw, "SECURITY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}

	if result.Findings[0].File != "a.go" || result.Findings[0].Line != 10 {
		t.Fatalf("unexpected finding: %+v", result.Findings[0])
	}
}

func TestParseResult_JSONWithCodeFence(t *testing.T) {
	raw := "```json\n{\"summary\":\"s\",\"findings\":[]}\n```"

	result, err := ParseResult(raw, "SECURITY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(result.Findings))
	}
}

func TestParseResult_JSONEmbeddedInText(t *testing.T) {
	raw := "Acá va la respuesta:\n{\"summary\":\"s\",\"findings\":[{\"file\":\"x.go\"}]}\nGracias."

	result, err := ParseResult(raw, "SECURITY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Findings) != 1 || result.Findings[0].File != "x.go" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestParseResult_Markdown(t *testing.T) {
	raw := "# Revisión\n\n" +
		"## Hallazgo 1\n\n" +
		"**archivo:** a.go\n" +
		"**línea:** 42\n" +
		"**categoría:** SECURITY\n" +
		"**comentario:** Ojo con esto.\n\n" +
		"## Hallazgo 2\n\n" +
		"**archivo:** b.go\n" +
		"**línea:** 7\n" +
		"**comentario:** Y con esto otro.\n"

	result, err := ParseResult(raw, "SECURITY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(result.Findings), result.Findings)
	}

	if result.Findings[0].File != "a.go" || result.Findings[0].Line != 42 {
		t.Fatalf("unexpected first finding: %+v", result.Findings[0])
	}

	if result.Findings[1].Category != "SECURITY" {
		t.Fatalf("expected default category to be applied, got %q", result.Findings[1].Category)
	}
}

func TestParseResult_UnrecognizedFormatIsError(t *testing.T) {
	if _, err := ParseResult("esto no es ni JSON ni Markdown reconocible", "SECURITY"); err == nil {
		t.Fatal("expected error for unrecognized format")
	}
}

func TestFallbackResult_PreservesRawResponse(t *testing.T) {
	raw := "respuesta rara"

	result := FallbackResult(raw, "SECURITY")

	if len(result.Findings) != 1 {
		t.Fatalf("expected exactly 1 fallback finding, got %d", len(result.Findings))
	}

	finding := result.Findings[0]

	if finding.Category != "SECURITY" {
		t.Fatalf("expected category SECURITY, got %q", finding.Category)
	}

	if len(finding.Details) != 1 || finding.Details[0].Value != raw {
		t.Fatalf("expected raw response preserved in details, got %+v", finding.Details)
	}
}

func TestRenderMarkdown_RoundTrip(t *testing.T) {
	result := &Result{
		Summary: "resumen",
		Findings: []Finding{
			{File: "a.go", Line: 1, Category: "SECURITY", Title: "t", Comment: "c", Suggestion: "s"},
		},
	}

	markdown := RenderMarkdown("Seguridad", result)

	reparsed, err := ParseResult(markdown, "SECURITY")
	if err != nil {
		t.Fatalf("unexpected error re-parsing rendered markdown: %v", err)
	}

	if len(reparsed.Findings) != 1 {
		t.Fatalf("expected 1 finding after round-trip, got %d", len(reparsed.Findings))
	}

	if reparsed.Findings[0].File != "a.go" || reparsed.Findings[0].Suggestion != "s" {
		t.Fatalf("round-trip lost data: %+v", reparsed.Findings[0])
	}
}
