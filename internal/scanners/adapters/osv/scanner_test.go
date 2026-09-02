package osv_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/osv-scanner/v2/pkg/models"
	osvschema "github.com/ossf/osv-schema/bindings/go/osvschema"

	osvAdapter "github.com/LucasNav6/code-review-cli/internal/scanners/adapters/osv"
	scannersdomain "github.com/LucasNav6/code-review-cli/internal/scanners/domain"
)

// TestNewReturnsSBOMScannerInterface is the contract test: New()
// returns the SBOMScanner port, never the concrete struct.
func TestNewReturnsSBOMScannerInterface(t *testing.T) {
	s := osvAdapter.New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
	var _ scannersdomain.SBOMScanner = s
}

// TestScan_EmptyRepoPathReturnsError verifies the fail-fast path:
// the adapter does NOT touch osv-scanner when given an empty
// repo path.
func TestScan_EmptyRepoPathReturnsError(t *testing.T) {
	s := osvAdapter.New()
	_, err := s.Scan(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty repo path, got nil")
	}
}

// TestConvert_HappyPath: one package with one CVE.
func TestConvert_HappyPath(t *testing.T) {
	v := &osvschema.Vulnerability{
		Id:      "GHSA-xxxx-yyyy-zzzz",
		Aliases: []string{"CVE-2024-12345"},
		Summary: "Bad thing happens",
		Details: "Long description here.",
		Severity: []*osvschema.Severity{
			{Type: osvschema.Severity_CVSS_V3, Score: "7.5"},
		},
		Affected: []*osvschema.Affected{
			{Ranges: []*osvschema.Range{
				{Events: []*osvschema.Event{{Fixed: "1.2.4"}}},
			}},
		},
	}
	raw := models.VulnerabilityResults{
		Results: []models.PackageSource{{
			Packages: []models.PackageVulns{{
				Package:         models.PackageInfo{Name: "com.example:lib", Version: "1.2.3"},
				Vulnerabilities: []*osvschema.Vulnerability{v},
			}},
		}},
	}
	out := osvAdapter.ConvertForTest("/tmp/repo", raw)

	if out.RepoPath != "/tmp/repo" {
		t.Errorf("RepoPath: got %q, want /tmp/repo", out.RepoPath)
	}
	if len(out.Vulnerabilities) != 1 {
		t.Fatalf("expected 1 vulnerability, got %d", len(out.Vulnerabilities))
	}
	got := out.Vulnerabilities[0]
	want := scannersdomain.Vulnerability{
		ID:          "GHSA-xxxx-yyyy-zzzz",
		Aliases:     []string{"CVE-2024-12345"},
		Summary:     "Bad thing happens",
		Details:     "Long description here.",
		Component:   "com.example:lib",
		Version:     "1.2.3",
		FixedVersion: "1.2.4",
		CVSSScore:   7.5,
		Severity:    scannersdomain.SeverityHigh,
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ID, want.ID)
	}
	if got.Component != want.Component {
		t.Errorf("Component: got %q, want %q", got.Component, want.Component)
	}
	if got.Version != want.Version {
		t.Errorf("Version: got %q, want %q", got.Version, want.Version)
	}
	if got.FixedVersion != want.FixedVersion {
		t.Errorf("FixedVersion: got %q, want %q", got.FixedVersion, want.FixedVersion)
	}
	if got.CVSSScore != want.CVSSScore {
		t.Errorf("CVSSScore: got %v, want %v", got.CVSSScore, want.CVSSScore)
	}
	if got.Severity != want.Severity {
		t.Errorf("Severity: got %s, want %s", got.Severity, want.Severity)
	}
	if len(got.Aliases) != 1 || got.Aliases[0] != "CVE-2024-12345" {
		t.Errorf("Aliases: got %v, want [CVE-2024-12345]", got.Aliases)
	}
}

// TestConvert_EmptyResult: no vulnerabilities found.
func TestConvert_EmptyResult(t *testing.T) {
	out := osvAdapter.ConvertForTest("/tmp/repo", models.VulnerabilityResults{})
	if out.RepoPath != "/tmp/repo" {
		t.Errorf("RepoPath: got %q", out.RepoPath)
	}
	if len(out.Vulnerabilities) != 0 {
		t.Errorf("expected 0 vulns, got %d", len(out.Vulnerabilities))
	}
}

// TestConvert_BucketSeverity covers the NVD-style thresholds.
// Each row is one bucket boundary. Changing these is a UX-visible
// change so it gets explicit coverage.
func TestConvert_BucketSeverity(t *testing.T) {
	cases := []struct {
		score float64
		want  scannersdomain.Severity
	}{
		{10.0, scannersdomain.SeverityCritical},
		{9.0, scannersdomain.SeverityCritical},
		{8.9, scannersdomain.SeverityHigh},
		{7.0, scannersdomain.SeverityHigh},
		{6.9, scannersdomain.SeverityMedium},
		{4.0, scannersdomain.SeverityMedium},
		{3.9, scannersdomain.SeverityLow},
		{0.1, scannersdomain.SeverityLow},
		{0.0, scannersdomain.SeverityUnknown},
	}
	for _, tc := range cases {
		t.Run(strconv.FormatFloat(tc.score, 'f', -1, 64), func(t *testing.T) {
			v := vulnWithScore(tc.score)
			out := osvAdapter.ConvertForTest("/tmp", models.VulnerabilityResults{
				Results: []models.PackageSource{{
					Packages: []models.PackageVulns{{
						Package:         models.PackageInfo{Name: "x"},
						Vulnerabilities: []*osvschema.Vulnerability{v},
					}},
				}},
			})
			if len(out.Vulnerabilities) != 1 {
				t.Fatalf("expected 1 vuln, got %d", len(out.Vulnerabilities))
			}
			if got := out.Vulnerabilities[0].Severity; got != tc.want {
				t.Errorf("score %v: got %s, want %s",
					tc.score, got, tc.want)
			}
		})
	}
}

// TestConvert_CVSSVectorKeptWhenNoScore: when the OSV record only
// has the vector (no numeric score), the adapter keeps the vector
// and leaves CVSSScore=0. The UI can then show "see vector".
func TestConvert_CVSSVectorKeptWhenNoScore(t *testing.T) {
	const vec = "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"
	v := &osvschema.Vulnerability{
		Id: "CVE-2024-X",
		Severity: []*osvschema.Severity{
			{Type: osvschema.Severity_CVSS_V3, Score: vec},
		},
	}
	out := osvAdapter.ConvertForTest("/tmp", models.VulnerabilityResults{
		Results: []models.PackageSource{{
			Packages: []models.PackageVulns{{
				Package:         models.PackageInfo{Name: "x"},
				Vulnerabilities: []*osvschema.Vulnerability{v},
			}},
		}},
	})
	got := out.Vulnerabilities[0]
	if got.CVSSScore != 0 {
		t.Errorf("CVSSScore must be 0 when only vector present, got %v", got.CVSSScore)
	}
	if got.CVSSVector != vec {
		t.Errorf("CVSSVector: got %q, want %q", got.CVSSVector, vec)
	}
}

// TestConvert_NoFixMeansEmptyFixedVersion: zero-days and
// unmaintained packages render without a FixedVersion.
func TestConvert_NoFixMeansEmptyFixedVersion(t *testing.T) {
	v := &osvschema.Vulnerability{Id: "CVE-2024-DAYZERO"}
	out := osvAdapter.ConvertForTest("/tmp", models.VulnerabilityResults{
		Results: []models.PackageSource{{
			Packages: []models.PackageVulns{{
				Package:         models.PackageInfo{Name: "x"},
				Vulnerabilities: []*osvschema.Vulnerability{v},
			}},
		}},
	})
	if got := out.Vulnerabilities[0].FixedVersion; got != "" {
		t.Errorf("FixedVersion must be empty when no fix event, got %q", got)
	}
}

// TestConvert_FirstFixWins: when multiple ranges advertise a fix,
// the adapter picks the first one. Stable across refactors.
func TestConvert_FirstFixWins(t *testing.T) {
	v := &osvschema.Vulnerability{
		Id: "CVE-2024-MULTI",
		Affected: []*osvschema.Affected{
			{Ranges: []*osvschema.Range{
				{Events: []*osvschema.Event{{Fixed: "1.2.4"}}},
			}},
			{Ranges: []*osvschema.Range{
				{Events: []*osvschema.Event{{Fixed: "2.0.0"}}},
			}},
		},
	}
	out := osvAdapter.ConvertForTest("/tmp", models.VulnerabilityResults{
		Results: []models.PackageSource{{
			Packages: []models.PackageVulns{{
				Package:         models.PackageInfo{Name: "x"},
				Vulnerabilities: []*osvschema.Vulnerability{v},
			}},
		}},
	})
	if got := out.Vulnerabilities[0].FixedVersion; got != "1.2.4" {
		t.Errorf("FixedVersion must be first fix (1.2.4), got %q", got)
	}
}

// vulnWithScore builds a synthetic vulnerability with a single
// numeric CVSS score. Used by the bucket-severity table.
func vulnWithScore(score float64) *osvschema.Vulnerability {
	return &osvschema.Vulnerability{
		Id: "CVE-TEST",
		Severity: []*osvschema.Severity{{
			Type:  osvschema.Severity_CVSS_V3,
			Score: strconv.FormatFloat(score, 'f', -1, 64),
		}},
	}
}