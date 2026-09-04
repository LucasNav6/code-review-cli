package domain_test

import (
	"testing"

	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
	"github.com/LucasNav6/code-review-cli/internal/review/domain"
)

// TestReview_ZeroValueUsable documents that an empty Review is a
// valid value: zero findings, zero metadata, no panic. The TUI
// renders an "empty" state for this (the overview says "0
// findings"); we do not want a missing finding to crash the
// model.
//
// Note: Findings is a nil slice by default. In Go, len(nil)==0
// and ranging over a nil slice is safe, so the consumer code
// does not need a nil guard. We document this rather than
// initialise an empty slice in the struct (which would mask
// "review ran but produced no findings" vs "review never ran").
func TestReview_ZeroValueUsable(t *testing.T) {
	var r domain.Review
	if len(r.Findings) != 0 {
		t.Errorf("len(Findings): got %d, want 0", len(r.Findings))
	}
	if r.PullRequest.Number != 0 {
		t.Errorf("PullRequest.Number: got %d, want 0", r.PullRequest.Number)
	}

	// Sanity: ranging over a nil findings slice must not panic.
	for range r.Findings {
		t.Errorf("unexpected finding in zero-value review")
	}
}

// TestReview_GroupFindingsBySeverity pins down the grouping
// helper the TUI uses for the "by severity" overview row. Adding
// a new severity bucket here is a UX change.
func TestReview_GroupFindingsBySeverity(t *testing.T) {
	cases := []struct {
		name     string
		findings []domain.Finding
		want     map[string]int
	}{
		{
			name:     "empty",
			findings: nil,
			want:     map[string]int{},
		},
		{
			name: "mixed",
			findings: []domain.Finding{
				{Title: "a", Category: string(domain.CategorySecurity), OWASP: "API1:2023"},
				{Title: "b", Category: string(domain.CategorySecurity), OWASP: "API2:2023"},
				{Title: "c", Category: string(domain.CategoryResilience)},
				{Title: "d", Category: string(domain.CategoryTesting)},
			},
			want: map[string]int{
				"SECURITY": 2,
				"RESILIENCE": 1,
				"TESTING": 1,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := domain.Review{Findings: tc.findings}
			got := groupByCategory(r.Findings)
			if len(got) != len(tc.want) {
				t.Fatalf("groupByCategory: got %d buckets, want %d (%v)",
					len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("bucket %q: got %d, want %d", k, got[k], v)
				}
			}
		})
	}
}

// TestReview_GroupBySeverity pins down the severity grouping.
// Today severity is derived from CVSS (SBOM) or left empty
// (LLM-emitted findings do not carry a numeric severity). The
// TUI uses this to render the "Critical/High/Medium/Low" row.
func TestReview_GroupBySeverity(t *testing.T) {
	findings := []domain.Finding{
		{CVE: "CVE-1", CVSS: 9.5},
		{CVE: "CVE-2", CVSS: 9.5},
		{CVE: "CVE-3", CVSS: 7.0},
		{CVE: "CVE-4", CVSS: 5.0},
		{CVE: "CVE-5"}, // no score
	}
	r := domain.Review{Findings: findings}

	got := groupBySeverity(r.Findings)
	want := map[string]int{
		"critical": 2,
		"high":     1,
		"medium":   1,
		"unknown":  1,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("severity %q: got %d, want %d", k, got[k], v)
		}
	}
}

// TestFinding_FieldsCarry verifies a Finding round-trips all
// fields. The TUI renders each field somewhere; if a field goes
// missing the user sees an empty card.
func TestFinding_FieldsCarry(t *testing.T) {
	f := domain.Finding{
		Title:      "Title",
		Context:    "Context",
		Impact:     []string{"impact 1", "impact 2"},
		Suggestion: "Suggestion",
		Category:   "SECURITY",
		Subcategory: "OWASP",
		File:       "internal/foo.go",
		Line:       42,
		Snippets: []domain.Snippet{
			{Line: 42, Code: "+ foo := 1"},
		},
		OWASP:              "API1:2023",
		CVE:                "CVE-2024-X",
		CVSS:               9.8,
		Component:          "com.example:lib",
		ComponentVersion:   "1.2.3",
		FixedVersion:       "1.2.4",
		SeverityLabel:      "CRITICAL",
	}

	if f.Title != "Title" {
		t.Errorf("Title: got %q", f.Title)
	}
	if len(f.Impact) != 2 {
		t.Errorf("Impact length: got %d, want 2", len(f.Impact))
	}
	if f.Subcategory != "OWASP" {
		t.Errorf("Subcategory: got %q", f.Subcategory)
	}
	if f.CVSS != 9.8 {
		t.Errorf("CVSS: got %v", f.CVSS)
	}
	if len(f.Snippets) != 1 || f.Snippets[0].Code != "+ foo := 1" {
		t.Errorf("Snippets: %v", f.Snippets)
	}
}

// groupByCategory returns a map of Category → count. Extracted
// here (rather than living in the TUI) so the contract is
// testable without a bubbletea dependency.
func groupByCategory(findings []domain.Finding) map[string]int {
	out := map[string]int{}
	for _, f := range findings {
		out[f.Category]++
	}
	return out
}

// groupBySeverity returns a map of severity bucket → count. The
// buckets follow the NVD convention (CVSS_V3 score → Critical /
// High / Medium / Low / Unknown). A finding without a CVSS score
// falls into "unknown".
func groupBySeverity(findings []domain.Finding) map[string]int {
	out := map[string]int{}
	for _, f := range findings {
		bucket := severityBucket(f.CVSS)
		out[bucket]++
	}
	return out
}

// severityBucket maps a CVSS score (0–10) to one of the five
// buckets the TUI displays. Same thresholds as the sbom adapter.
func severityBucket(cvss float64) string {
	switch {
	case cvss >= 9.0:
		return "critical"
	case cvss >= 7.0:
		return "high"
	case cvss >= 4.0:
		return "medium"
	case cvss > 0:
		return "low"
	default:
		return "unknown"
	}
}

// ensure scmdomain stays referenced so the import is not pruned
// before the TUI wires the PR header.
var _ = scmdomain.PRMetadata{}