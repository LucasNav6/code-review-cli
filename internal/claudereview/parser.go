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

// Finding is one resilience observation emitted by claude. Fields are
// exported so the renderer can address them by name without coupling
// to the wire format.
type Finding struct {
	Title       string    `json:"title"`
	Context     string    `json:"context"`
	Impact      []string  `json:"impact"`
	Suggestion  string    `json:"suggestion"`
	Category    string    `json:"category"`
	File        string    `json:"file"`
	Line        int       `json:"line"`
	Snippets    []Snippet `json:"snippets"`
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

// Parse converts the raw claude response into a list of Finding.
// Returns ErrMalformedResponse if the payload is not JSON,
// ErrNoFindingsField if the JSON is valid but missing the findings
// key, and ErrMalformedFinding if any finding lacks required fields.
func Parse(response string) ([]Finding, error) {
	response = strings.TrimSpace(response)
	if response == "" {
		return nil, helpers.ErrEmptyResponse
	}

	// Some claude runs wrap the JSON in markdown fences despite the
	// prompt telling it not to. Strip them defensively before parsing.
	response = stripCodeFences(response)

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

// normaliseFinding trims whitespace and fills defaults for fields
// that claude occasionally omits. Doing it here keeps the renderer
// free of nil checks.
func normaliseFinding(f Finding) Finding {
	f.Title = strings.TrimSpace(f.Title)
	f.Context = strings.TrimSpace(f.Context)
	f.Suggestion = strings.TrimSpace(f.Suggestion)
	f.File = strings.TrimSpace(f.File)
	f.Category = strings.TrimSpace(f.Category)
	if f.Category == "" {
		f.Category = "RESILIENCE"
	}

	for i, item := range f.Impact {
		f.Impact[i] = strings.TrimSpace(item)
	}
	for i, s := range f.Snippets {
		f.Snippets[i].Code = strings.TrimSpace(s.Code)
	}
	return f
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
