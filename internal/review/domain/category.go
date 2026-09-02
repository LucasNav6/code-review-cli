package domain

import "strings"

// Category is one of the four review passes the LLM can run against
// the diff. The string values mirror the JSON field the prompts ask
// the LLM to emit ("category": "RESILIENCE"), so this type can be
// compared against a parsed Finding.Category without translation.
//
// Keep the list closed: adding a category means a new prompt file, a
// new mapping in prompt.go, and a new branch in the renderer.
type Category string

const (
	CategoryResilience      Category = "RESILIENCE"
	CategoryMaintainability Category = "READABILITY"
	CategorySecurity        Category = "SECURITY"
	CategoryTesting         Category = "TESTING"
)

// Canonical returns the upper-cased Category. Unknown inputs collapse
// to CategoryResilience, matching the parser's fallback behaviour so
// the domain and the parser agree on what counts as "unknown".
func (c Category) Canonical() Category {
	upper := Category(strings.ToUpper(string(c)))
	switch upper {
	case CategoryResilience, CategoryMaintainability, CategorySecurity, CategoryTesting:
		return upper
	}
	return CategoryResilience
}

// IsKnown reports whether c is one of the four documented categories.
// Callers use it to gate UI affordances (e.g. showing the OWASP block
// only for CategorySecurity).
func (c Category) IsKnown() bool {
	switch c {
	case CategoryResilience, CategoryMaintainability, CategorySecurity, CategoryTesting:
		return true
	}
	return false
}

// ReviewType is the value the user passes via --type. It is distinct
// from Category because the user can pass the sentinel "all" to mean
// "every category in canonical order", which is not itself a category.
type ReviewType string

const (
	ReviewTypeAll            ReviewType = "all"
	ReviewTypeResilience     ReviewType = "resilience"
	ReviewTypeMaintainability ReviewType = "maintainability"
	ReviewTypeSecurity       ReviewType = "security"
	ReviewTypeTesting        ReviewType = "testing"
)

// IsKnown reports whether t is a recognised ReviewType value. The
// empty string is NOT considered valid: callers should resolve empty
// to the default (ReviewTypeAll) before calling ResolveCategories.
func (t ReviewType) IsKnown() bool {
	switch t {
	case ReviewTypeAll,
		ReviewTypeResilience,
		ReviewTypeMaintainability,
		ReviewTypeSecurity,
		ReviewTypeTesting:
		return true
	}
	return false
}

// DefaultReviewType is the value used when the user did not pass
// --type. We default to "all" so a bare `code-review review --url
// <pr>` invocation runs the four canonical passes in order. Passing
// --type explicitly remains the way to scope the run to a single
// category (useful for CI, for iterating on a single prompt, or to
// keep token usage low).
const DefaultReviewType = ReviewTypeAll

// CanonicalOrder is the stable order in which categories are
// presented to the user when --type all is used. The order is the
// order shown on screen, so changing it is a UX change and should be
// done deliberately.
func CanonicalOrder() []Category {
	return []Category{
		CategoryResilience,
		CategoryMaintainability,
		CategorySecurity,
		CategoryTesting,
	}
}

// ResolveCategories maps a ReviewType to the list of Categories that
// must be executed for that invocation.
//
//   - ReviewTypeAll expands to the four canonical categories in
//     CanonicalOrder.
//   - A single-category ReviewType resolves to a one-element list.
//   - An empty string resolves to the default (all) and never errors:
//     the cmd layer treats empty as "user did not pass --type", which
//     is a valid input.
//   - Any other value returns (nil, ErrUnknownReviewType) so the
//     caller can decide whether to fail hard or fall back.
//
// The function is pure: no I/O, no logging, no global state. That is
// why it lives here and not in the cmd layer.
func ResolveCategories(t ReviewType) ([]Category, error) {
	switch t {
	case "", ReviewTypeAll:
		return CanonicalOrder(), nil
	case ReviewTypeResilience:
		return []Category{CategoryResilience}, nil
	case ReviewTypeMaintainability:
		return []Category{CategoryMaintainability}, nil
	case ReviewTypeSecurity:
		return []Category{CategorySecurity}, nil
	case ReviewTypeTesting:
		return []Category{CategoryTesting}, nil
	}
	return nil, ErrUnknownReviewType
}