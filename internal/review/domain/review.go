package domain

import scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"

// Review is the result of one Execute call. It is the single
// return value the bubbletea TUI (F4-F7) renders. Today the use
// case paints the same data incrementally via OutputSink; F3
// will refactor Execute to return this struct instead.
//
// Putting Review in the domain (not in claudereview or anywhere
// upstream of the use case) keeps the contract small: it is the
// exact slice of data the TUI needs, no more. Adding fields here
// is a UX change, not a plumbing change.
type Review struct {
	// PullRequest is the metadata the TUI shows in the header /
	// overview. It comes from scm.FetchMetadata; the use case
	// passes it through verbatim.
	PullRequest scmdomain.PRMetadata

	// Findings is the flat list across all categories, in the
	// order the use case produced them. The TUI groups them by
	// severity / category / file at render time (groupBy* is a
	// pure function on top of this slice).
	//
	// Today the source is the LLM response, parsed by
	// claudereview.Parse. F3 may also include SBOM-derived
	// findings (CVE-driven) under SECURITY:SBOM category; the
	// struct shape is identical so the TUI does not need to
	// special-case them.
	Findings []Finding
}

// Finding is the domain-level shape of one review observation.
// It is the JSON shape the LLM emits (after claudereview.Parse
// normalises category casing + extracts sub-category) augmented
// with the rendered file:line and the SBOM-specific fields.
//
// Today the same struct lives in internal/claudereview.Finding;
// we re-declare it here so the TUI (and future consumers) can
// depend on the domain package without pulling in the LLM wire
// format. F3 will move the canonical type into this package and
// make claudereview re-export it for backward compatibility.
//
// Keeping the struct flat (no nested objects) keeps grouping +
// filtering in the TUI trivial.
type Finding struct {
	Title      string
	Context    string
	Impact     []string
	Suggestion string

	// Category is the parent bucket (one of the four review
	// categories or "SECURITY:SBOM"). Subcategory is the optional
	// child bucket extracted from the Category string at parse
	// time. SECURITY findings have Subcategory == "".
	Category    string
	Subcategory string

	// File:Line identify the location in the diff the finding
	// refers to. Line == 0 when the finding is file-level
	// (e.g. a missing test for a whole module).
	File string
	Line int

	// Snippets is the raw code the LLM quoted. The TUI renders
	// the first snippet in the detail view; the rest are
	// available for future "show full context" affordances.
	Snippets []Snippet

	// SECURITY-only: OWASP API Security Top 10 category, e.g.
	// "API1:2023". Empty for non-security findings.
	OWASP string

	// SECURITY:SBOM-only: CVE / CVSS / Component fields populated
	// from osv-scanner (the LLM does NOT invent them; it just
	// surfaces what the scanner found).
	CVE              string
	CVSS             float64
	Component        string
	ComponentVersion string
	FixedVersion     string
	SeverityLabel    string
}

// Snippet is one block of source code that illustrates a finding.
// The Code field carries the original diff line with its prefix
// (+, -, space) preserved.
type Snippet struct {
	Line int
	Code string
}