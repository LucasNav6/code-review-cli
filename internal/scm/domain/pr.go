package domain

// PRMetadata is the small subset of pull-request fields the CLI
// needs to render the header box (title, number, state, branches,
// author). The struct is intentionally tiny — anything not used by
// the header should not live here.
//
// JSON tags mirror the shape returned by `gh pr view --json` so the
// gh adapter can decode straight into this type without an
// intermediate map. Other adapters (git, hosted APIs) define their
// own wire format and translate into this struct before returning.
type PRMetadata struct {
	Number      int
	Title       string
	State       string // "OPEN", "MERGED", "CLOSED"
	IsDraft     bool
	HeadRefName string
	BaseRefName string
	AuthorLogin string
}