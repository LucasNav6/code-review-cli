package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/LucasNav6/code-review-cli/internal/review"
)

func TestParseHunkHeader(t *testing.T) {
	oldLine, newLine, ok := parseHunkHeader("@@ -10,7 +20,9 @@ func run()")
	if !ok {
		t.Fatal("expected hunk header to parse")
	}

	if oldLine != 10 || newLine != 20 {
		t.Fatalf("expected old=10 new=20, got old=%d new=%d", oldLine, newLine)
	}
}

func TestRenderInlineReviewIncludesDiffAndComment(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/internal/app.go b/internal/app.go",
		"index 1111111..2222222 100644",
		"--- a/internal/app.go",
		"+++ b/internal/app.go",
		"@@ -1,3 +1,4 @@",
		" package internal",
		"+func risky() {}",
		" func stable() {}",
		"",
	}, "\n")

	findings := []review.Finding{
		{
			File:     "internal/app.go",
			Line:     2,
			Category: "security",
			Title:    "Avoid risky helper",
			Comment:  "This helper has no validation.",
		},
	}

	rendered := stripANSI(renderInlineReview(diff, findings, 100))

	for _, expected := range []string{
		"internal/app.go",
		"+ func risky() {}",
		"SECURITY: Avoid risky helper",
		"This helper has no validation.",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected rendered output to contain %q:\n%s", expected, rendered)
		}
	}

	codeIndex := strings.Index(rendered, "+ func risky() {}")
	commentIndex := strings.Index(rendered, "SECURITY: Avoid risky helper")
	if codeIndex == -1 || commentIndex == -1 || commentIndex < codeIndex {
		t.Fatalf("expected comment to render inline after target line:\n%s", rendered)
	}
}

func TestRenderFilesChangedAddsMockInlineComment(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/internal/app.go b/internal/app.go",
		"--- a/internal/app.go",
		"+++ b/internal/app.go",
		"@@ -1,2 +1,3 @@",
		" package internal",
		"+func mockTarget() {}",
		" func stable() {}",
		"",
	}, "\n")

	rendered := stripANSI(renderFilesChanged(diff, nil, 100))

	for _, expected := range []string{
		"+ func mockTarget() {}",
		"func stable() {}",
		"OWASP TOP 10: Rule #1",
		"Este es el lugar donde aparecería",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected rendered output to contain %q:\n%s", expected, rendered)
		}
	}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiRe.ReplaceAllString(value, "")
}
