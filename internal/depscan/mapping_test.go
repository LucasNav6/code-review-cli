package depscan

import (
	"strings"
	"testing"

	"github.com/google/osv-scanner/v2/pkg/models"
	"github.com/ossf/osv-schema/bindings/go/osvschema"
)

func TestSeverityLabel(t *testing.T) {
	cases := map[string]string{
		"9.8":  "CRITICAL",
		"9.0":  "CRITICAL",
		"7.5":  "HIGH",
		"7.0":  "HIGH",
		"5.0":  "MEDIUM",
		"4.0":  "MEDIUM",
		"1.0":  "LOW",
		"0.1":  "LOW",
		"0.0":  "UNKNOWN",
		"":     "UNKNOWN",
		"nope": "UNKNOWN",
	}

	for score, want := range cases {
		if got := severityLabel(score); got != want {
			t.Errorf("severityLabel(%q) = %q, want %q", score, got, want)
		}
	}
}

func TestFixedVersionHint(t *testing.T) {
	vuln := &osvschema.Vulnerability{
		Id: "GHSA-xxxx",
		Affected: []*osvschema.Affected{
			{
				Ranges: []*osvschema.Range{
					{
						Events: []*osvschema.Event{
							{Introduced: "0"},
							{Fixed: "1.2.3"},
						},
					},
				},
			},
		},
	}

	if got := fixedVersionHint(vuln); got != "1.2.3" {
		t.Fatalf("fixedVersionHint() = %q, want %q", got, "1.2.3")
	}

	unfixed := &osvschema.Vulnerability{Id: "GHSA-yyyy"}

	if got := fixedVersionHint(unfixed); got != "" {
		t.Fatalf("fixedVersionHint() on unfixed vuln = %q, want empty", got)
	}
}

func TestBuildResult_MapsFindingsAndOrdersBySeverity(t *testing.T) {
	results := models.VulnerabilityResults{
		Results: []models.PackageSource{
			{
				Source: models.SourceInfo{Path: "/scan/package-lock.json"},
				Packages: []models.PackageVulns{
					{
						Package: models.PackageInfo{Name: "lodash", Version: "4.17.15", Ecosystem: "npm"},
						Vulnerabilities: []*osvschema.Vulnerability{
							{
								Id:      "GHSA-low",
								Summary: "Low severity issue",
							},
							{
								Id:      "GHSA-critical",
								Summary: "Critical severity issue",
								Affected: []*osvschema.Affected{
									{Ranges: []*osvschema.Range{{Events: []*osvschema.Event{{Fixed: "4.17.21"}}}}},
								},
							},
						},
						Groups: []models.GroupInfo{
							{IDs: []string{"GHSA-low"}, MaxSeverity: "2.0"},
							{IDs: []string{"GHSA-critical"}, MaxSeverity: "9.8"},
						},
					},
				},
			},
		},
	}

	result, raw := buildResult("/scan", results)

	if len(result.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(result.Findings), result.Findings)
	}

	if result.Findings[0].Category != "CRITICAL" {
		t.Fatalf("expected the CRITICAL finding first, got %q", result.Findings[0].Category)
	}

	if result.Findings[1].Category != "LOW" {
		t.Fatalf("expected the LOW finding second, got %q", result.Findings[1].Category)
	}

	if !strings.Contains(result.Findings[0].File, "package-lock.json") {
		t.Fatalf("expected finding to reference the lockfile, got %q", result.Findings[0].File)
	}

	if !strings.Contains(result.Findings[0].Suggestion, "4.17.21") {
		t.Fatalf("expected suggestion to mention the fixed version, got %q", result.Findings[0].Suggestion)
	}

	if !strings.Contains(raw, "CRITICAL") {
		t.Fatalf("expected raw summary to mention severities, got %q", raw)
	}
}

func TestBuildResult_NoVulnerabilities(t *testing.T) {
	result, _ := buildResult("/scan", models.VulnerabilityResults{})

	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(result.Findings))
	}
}
