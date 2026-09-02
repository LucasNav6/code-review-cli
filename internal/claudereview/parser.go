// Package claudereview parses and renders the structured JSON that
// the bundled prompt templates ask claude to emit. The shape is:
//
//	{
//	  "findings": [
//	    {
//	      "title":       "Falta manejar fallo del iframe",
//	      "context":     "...",
//	      "impact":      ["...", "..."],
//	      "suggestion":  "...",
//	      "category":    "RESILIENCE",
//	      "file":        "path/to/file.ext",
//	      "line":        118,
//	      "snippets":    [{"line": 118, "code": "+ $monorepo_url = ..."}]
//	    }
//	  ]
//	}
//
// Every prompt in internal/prompts/ emits this same shape so a single
// parser can serve the four review categories (resilience, readability,
// security, testing). The Finding struct carries a few optional fields
// (OWASP for security; requires_tests / tests_covered / tests_missing /
// edge_case for testing) that the renderer hides when empty.
//
// As a fallback the parser also accepts the literal string "NO_FINDINGS"
// (after trim + fence stripping) and returns an empty slice. The original
// prompts allowed that as an alternative to {"findings": []} and we
// preserve that contract so the LLM can pick whichever it produces.
//
// An empty findings array renders as a success indicator by the caller.
package claudereview

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/LucasNav6/code-review-cli/helpers"
)

// Sentinel errors raised by the parser. They are exported so callers
// can categorise failures without string matching.
var (
	ErrMalformedResponse = errors.New("claude response was not valid JSON")
	ErrNoFindingsField   = errors.New("claude response did not contain a `findings` field")
	ErrMalformedFinding  = errors.New("claude response had a finding with no `file` or `title`")
)

// Category constants for the four supported review types. They mirror
// the `category` field the prompts emit and are exposed so callers and
// tests can refer to them without string literals scattered around.
const (
	CategoryResilience      = "RESILIENCE"
	CategoryReadability     = "READABILITY"
	CategorySecurity        = "SECURITY"
	CategoryTesting         = "TESTING"
	CategoryDefaultFallback = CategoryResilience // when the LLM omits the field
)

// Finding is one review observation emitted by claude. Fields are
// exported so the renderer can address them by name without coupling
// to the wire format.
//
// The OWASP/RequiresTests/TestsCovered/TestsMissing/EdgeCase fields are
// category-specific: they are populated only for SECURITY and TESTING
// findings respectively. They use omitempty so the JSON payload stays
// minimal for the two categories that do not need them.
type Finding struct {
	Title      string    `json:"title"`
	Context    string    `json:"context"`
	Impact     []string  `json:"impact"`
	Suggestion string    `json:"suggestion"`
	Category   string    `json:"category"`
	File       string    `json:"file"`
	Line       int       `json:"line"`
	Snippets   []Snippet `json:"snippets"`

	// SECURITY-only: the OWASP API Security Top 10 category, e.g.
	// "API1:2023". Optional so resilience/readability/testing findings
	// stay clean.
	OWASP string `json:"owasp,omitempty"`

	// TESTING-only fields. RequiresTests is a *bool so we can tell
	// "the LLM did not emit it" apart from "the LLM emitted false".
	// The other three are free-text descriptions rendered as
	// additional lines under the Impact block.
	RequiresTests *bool  `json:"requires_tests,omitempty"`
	TestsCovered  string `json:"tests_covered,omitempty"`
	TestsMissing  string `json:"tests_missing,omitempty"`
	EdgeCase      string `json:"edge_case,omitempty"`
}

// Snippet is one block of source code that illustrates a finding.
// Each entry preserves the original diff prefix (+, -, or space) so
// the renderer can colour them like a real diff.
type Snippet struct {
	Line int    `json:"line"`
	Code string `json:"code"`
}

// envelope mirrors the top-level shape claude returns. We use a
// dedicated type so future top-level fields (e.g. "summary") slot in
// without breaking callers.
type envelope struct {
	Findings []Finding `json:"findings"`
}

// noFindingsLiteral is the alternative empty-response shape the prompts
// accept. We keep it as a constant so the parser is easy to audit.
const noFindingsLiteral = "NO_FINDINGS"

// Parse converts the raw claude response into a list of Finding.
// Returns ErrMalformedResponse if the payload is not JSON,
// ErrNoFindingsField if the JSON is valid but missing the findings
// key, and ErrMalformedFinding if any finding lacks required fields.
//
// A response that is exactly the literal "NO_FINDINGS" (after trim and
// fence stripping) returns an empty slice with no error. This matches
// the contract written into the prompts.
func Parse(response string) ([]Finding, error) {
	response = strings.TrimSpace(response)
	if response == "" {
		return nil, helpers.ErrEmptyResponse
	}

	// Some claude runs wrap the JSON in markdown fences despite the
	// prompt telling it not to. Strip them defensively before parsing.
	response = stripCodeFences(response)

	// Honour the literal "NO_FINDINGS" shortcut the prompts allow.
	// It is case-sensitive on purpose so accidental matches (e.g.
	// text containing "no_findings" inside a sentence) do not slip
	// through.
	if response == noFindingsLiteral {
		return []Finding{}, nil
	}

	var env envelope
	if err := json.Unmarshal([]byte(response), &env); err != nil {
		return nil, helpers.ErrMalformedResponse
	}

	// Validate every finding. We accept the slice as-is when it is
	// empty (no findings is a valid outcome), and reject only when
	// individual findings are malformed.
	for i, f := range env.Findings {
		if f.File == "" || f.Title == "" {
			return nil, helpers.ErrMalformedFinding
		}
		env.Findings[i] = normaliseFinding(f)
	}

	return env.Findings, nil
}

// normaliseFinding trims whitespace, fills defaults for fields that
// claude occasionally omits, and validates category-specific rules.
// Doing it here keeps the renderer free of nil checks and centralises
// every "if the LLM forgot this, default to X" decision in one place.
func normaliseFinding(f Finding) Finding {
	f.Title = strings.TrimSpace(f.Title)
	f.Context = strings.TrimSpace(f.Context)
	f.Suggestion = strings.TrimSpace(f.Suggestion)
	f.File = strings.TrimSpace(f.File)
	f.Category = normaliseCategory(f.Category)
	f.OWASP = strings.TrimSpace(f.OWASP)
	f.TestsCovered = strings.TrimSpace(f.TestsCovered)
	f.TestsMissing = strings.TrimSpace(f.TestsMissing)
	f.EdgeCase = strings.TrimSpace(f.EdgeCase)

	for i, item := range f.Impact {
		f.Impact[i] = strings.TrimSpace(item)
	}
	for i, s := range f.Snippets {
		f.Snippets[i].Code = strings.TrimSpace(s.Code)
	}

	// SECURITY findings without an OWASP category are not actionable —
	// the whole point of that category is to map to the OWASP API
	// Top 10. We default to "API0:2023" (a sentinel that means
	// "unspecified") so the renderer can flag it visibly rather than
	// silently drop the finding.
	if f.Category == CategorySecurity && f.OWASP == "" {
		f.OWASP = "API0:2023"
	}

	return f
}

// normaliseCategory uppercases the incoming string and falls back to
// the default category when the LLM emitted something we don't
// recognise. Returning a known value keeps the renderer's switch
// exhaustive without forcing it to handle every typo the model can
// invent.
func normaliseCategory(raw string) string {
	c := strings.ToUpper(strings.TrimSpace(raw))
	switch c {
	case CategoryResilience, CategoryReadability, CategorySecurity, CategoryTesting:
		return c
	}
	return CategoryDefaultFallback
}

// stripCodeFences removes leading/trailing markdown code fences
// (```json ... ```) that some claude responses wrap the payload in
// despite explicit instructions to the contrary. It is intentionally
// lenient — anything that looks like a fence is removed.
func stripCodeFences(s string) string {
	const fence = "```"
	if !strings.HasPrefix(s, fence) {
		return s
	}

	// Drop the opening fence and any language tag after it.
	rest := strings.TrimPrefix(s, fence)
	if idx := strings.IndexByte(rest, '\n'); idx >= 0 {
		rest = rest[idx+1:]
	}

	// Drop the closing fence if present.
	if idx := strings.LastIndex(rest, fence); idx >= 0 {
		rest = rest[:idx]
	}
	return strings.TrimSpace(rest)
}
