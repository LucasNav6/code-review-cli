package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/LucasNav6/code-review-cli/internal/review/tui"
	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

// sampleReview returns a Review with a known distribution of
// findings: 1 critical, 1 high, 1 medium (all security) plus 1
// SBOM (critical via CVSS). The tests pin the model's grouping
// behaviour against this fixture.
func sampleReview() reviewdomain.Review {
	return reviewdomain.Review{
		PullRequest: scmdomain.PRMetadata{
			Number: 5323, Title: "Release 2026-09-01", State: "MERGED",
			AuthorLogin: "yagoz",
			HeadRefName: "release/2026-09-01", BaseRefName: "master",
		},
		Findings: []reviewdomain.Finding{
			{
				Title: "Critical: SQL injection", Category: "SECURITY",
				OWASP: "API1:2023",
			},
			{
				Title: "High: missing auth", Category: "SECURITY",
				OWASP: "API5:2023",
			},
			{
				Title: "Medium: leaky log", Category: "RESILIENCE",
			},
			{
				Title: "SBOM: bad lib", Category: "SECURITY:SBOM",
				CVE: "CVE-2024-X", CVSS: 9.5,
			},
		},
	}
}

// viewString converts the bubbletea View to a plain string so
// substring assertions in tests read naturally. tea.View is a
// struct that contains the rendered string content; we extract it
// via fmt to avoid depending on the struct's private layout.
func viewString(v tea.View) string {
	return fmt.Sprintf("%v", v)
}

// TestNew_StartsInOverview pins down the initial state.
func TestNew_StartsInOverview(t *testing.T) {
	m := tui.New(sampleReview())
	out := viewString(m.View())
	for _, want := range []string{
		"Release 2026-09-01", // PR title
		"#5323",               // PR number
		"yagoz",               // author
		"release/2026-09-01",  // head
		"master",              // base
		"Review complete",
		"Critical",
		"High",
		"Medium",
		"Low",
		"Security",
		"Resilience",
		"Readability",
		"Testing",
		"Dependencies",
		"press q to quit",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("overview missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestNew_ShowsFindingCount pins down the total count line.
func TestNew_ShowsFindingCount(t *testing.T) {
	m := tui.New(sampleReview())
	out := viewString(m.View())
	if !strings.Contains(out, "4 findings") {
		t.Errorf("expected '4 findings' in output, got:\n%s", out)
	}
}

// TestUpdate_WindowSizeSurvives verifies Update does not panic
// for the most common bubbletea message.
func TestUpdate_WindowSizeSurvives(t *testing.T) {
	m := tui.New(sampleReview())
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
}

// TestNewError_RendersError verifies that an error review shows
// the error message + the "press q to quit" hint.
func TestNewError_RendersError(t *testing.T) {
	m := tui.NewError("gh CLI not installed")
	out := viewString(m.View())
	for _, want := range []string{"error", "gh CLI not installed", "press q to quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("error view missing %q\n--- output ---\n%s", want, out)
		}
	}
}

// TestCountByCategory_StableOrder pins down the "by category"
// row order. Adding/removing a category here is a UX-visible
// change that requires updating this test.
func TestCountByCategory_StableOrder(t *testing.T) {
	m := tui.New(sampleReview())
	out := viewString(m.View())

	// Security must come before Resilience (more severe first).
	secIdx := strings.Index(out, "Security")
	resIdx := strings.Index(out, "Resilience")
	if secIdx < 0 || resIdx < 0 {
		t.Fatalf("categories missing from output:\n%s", out)
	}
	if secIdx >= resIdx {
		t.Errorf("Security must render before Resilience; got Security@%d Resilience@%d",
			secIdx, resIdx)
	}
}