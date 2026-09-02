package domain_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/LucasNav6/code-review-cli/internal/review/domain"
)

// TestResolveCategories_KnownTypes pins down the mapping for every
// documented --type value. The "all" case is the most important: it
// drives the default behaviour of `code-review review` and the order
// must stay stable because it is the order shown to the user.
func TestResolveCategories_KnownTypes(t *testing.T) {
	cases := []struct {
		name string
		in   domain.ReviewType
		want []domain.Category
	}{
		{
			name: "empty defaults to all",
			in:   "",
			want: domain.CanonicalOrder(),
		},
		{
			name: "explicit all",
			in:   domain.ReviewTypeAll,
			want: []domain.Category{
				domain.CategoryResilience,
				domain.CategoryMaintainability,
				domain.CategorySecurity,
				domain.CategoryTesting,
			},
		},
		{
			name: "resilience",
			in:   domain.ReviewTypeResilience,
			want: []domain.Category{domain.CategoryResilience},
		},
		{
			name: "maintainability",
			in:   domain.ReviewTypeMaintainability,
			want: []domain.Category{domain.CategoryMaintainability},
		},
		{
			name: "security",
			in:   domain.ReviewTypeSecurity,
			want: []domain.Category{domain.CategorySecurity},
		},
		{
			name: "testing",
			in:   domain.ReviewTypeTesting,
			want: []domain.Category{domain.CategoryTesting},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.ResolveCategories(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ResolveCategories(%q):\n got  %#v\n want %#v",
					tc.in, got, tc.want)
			}
		})
	}
}

// TestResolveCategories_Unknown verifies that anything that is not a
// documented ReviewType returns ErrUnknownReviewType and an empty
// slice. The cmd layer uses this signal to decide between "fail hard"
// and "log a warning + fall back".
func TestResolveCategories_Unknown(t *testing.T) {
	cases := []domain.ReviewType{
		"resilince",    // typo
		"ALL",          // case matters: --type is case-sensitive
		"readability",  // old name, replaced by maintainability
		"foo",
		" ",
	}
	for _, in := range cases {
		t.Run(string(in), func(t *testing.T) {
			got, err := domain.ResolveCategories(in)
			if got != nil {
				t.Errorf("expected nil slice on unknown input, got %#v", got)
			}
			if !errors.Is(err, domain.ErrUnknownReviewType) {
				t.Errorf("expected ErrUnknownReviewType, got %v", err)
			}
		})
	}
}

// TestCanonicalOrderStable locks down the on-screen ordering. The
// renderer uses this order to print the section dividers between
// passes; changing it is a UX-visible change so it gets its own test.
func TestCanonicalOrderStable(t *testing.T) {
	want := []domain.Category{
		domain.CategoryResilience,
		domain.CategoryMaintainability,
		domain.CategorySecurity,
		domain.CategoryTesting,
	}
	got := domain.CanonicalOrder()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CanonicalOrder drift:\n got  %#v\n want %#v", got, want)
	}
}

// TestCategoryCanonical_RoundTrip verifies the normalisation logic:
// any casing of a known category resolves to itself, and unknown
// values collapse to RESILIENCE (matching the parser's behaviour).
func TestCategoryCanonical_RoundTrip(t *testing.T) {
	cases := []struct {
		in   domain.Category
		want domain.Category
	}{
		{domain.CategoryResilience, domain.CategoryResilience},
		{domain.CategorySecurity, domain.CategorySecurity},
		{"security", domain.CategorySecurity},
		{"SECURITY", domain.CategorySecurity},
		{"SecurITy", domain.CategorySecurity},
		{"resilince", domain.CategoryResilience}, // typo → fallback
		{"", domain.CategoryResilience},
		{"FOO", domain.CategoryResilience},
	}
	for _, tc := range cases {
		t.Run(string(tc.in), func(t *testing.T) {
			if got := tc.in.Canonical(); got != tc.want {
				t.Errorf("Canonical(%q): got %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestPromptForCategory verifies every category maps to its prompt
// file. Unknown categories fall back to resilience (same fallback
// rule as the parser) so a typo never silently drops a pass.
func TestPromptForCategory(t *testing.T) {
	cases := []struct {
		cat domain.Category
		want domain.PromptFile
	}{
		{domain.CategoryResilience, domain.PromptResilience},
		{domain.CategoryMaintainability, domain.PromptMaintainability},
		{domain.CategorySecurity, domain.PromptSecurity},
		{domain.CategoryTesting, domain.PromptTesting},
		{"FOO", domain.PromptResilience}, // unknown → fallback
	}
	for _, tc := range cases {
		t.Run(string(tc.cat), func(t *testing.T) {
			if got := domain.PromptForCategory(tc.cat); got != tc.want {
				t.Errorf("PromptForCategory(%q): got %q, want %q",
					tc.cat, got, tc.want)
			}
		})
	}
}

// TestPromptFilePath ensures PromptFile.Path() always returns a path
// inside PromptDir. This is the contract the loader adapter relies
// on.
func TestPromptFilePath(t *testing.T) {
	want := domain.PromptDir + "/" + string(domain.PromptResilience)
	if got := domain.PromptResilience.Path(); got != want {
		t.Errorf("PromptResilience.Path(): got %q, want %q", got, want)
	}
}

// TestDefaultReviewType guards the documented default. A test beats
// a grep-and-hope when somebody refactors this in two years.
func TestDefaultReviewType(t *testing.T) {
	if domain.DefaultReviewType != domain.ReviewTypeAll {
		t.Fatalf("DefaultReviewType drift: got %q, want %q",
			domain.DefaultReviewType, domain.ReviewTypeAll)
	}
}