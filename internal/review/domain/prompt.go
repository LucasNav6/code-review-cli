package domain

// PromptDir is the directory (relative to the process working
// directory) where the bundled prompt templates live. Each
// `<category>.md` file is loaded at run time and substituted with the
// cached diff via the {{DIFF}} placeholder.
//
// The path is kept relative (not absolute) so the same binaries work
// both when invoked from a checkout (`go run .`) and when installed
// via the install script (which copies the prompts/ folder next to
// the binary). The loader that turns a PromptFile into bytes lives
// in the prompts/ adapters package; this domain package only knows
// about the names.
const PromptDir = "internal/prompts"

// PromptFile is the bare filename (no directory) of a prompt
// template. We keep it as a string type so the type system catches
// the classic typo "resiliance.md".
type PromptFile string

const (
	PromptResilience      PromptFile = "resilience.md"
	PromptMaintainability PromptFile = "maintainability.md"
	PromptSecurity        PromptFile = "security.md"
	PromptSecuritySBOM    PromptFile = "security_sbom.md"
	PromptTesting         PromptFile = "testing.md"
)

// Path returns the absolute-or-cwd-relative path of the prompt file,
// ready to be passed to a loader that resolves it against the
// filesystem. The caller decides what "relative to" means; the domain
// does not.
func (p PromptFile) Path() string {
	return PromptDir + "/" + string(p)
}

// PromptForCategory returns the prompt file associated with the given
// category. It is the canonical source for the mapping — callers MUST
// NOT hardcode "resilience.md" anywhere else; if you find yourself
// wanting to, extend this method instead.
//
// Unknown categories fall back to PromptResilience so the CLI never
// silently skips a pass when the LLM emits a typo'd category label.
func PromptForCategory(c Category) PromptFile {
	switch c {
	case CategoryMaintainability:
		return PromptMaintainability
	case CategorySecurity:
		return PromptSecurity
	case CategorySecuritySBOM:
		return PromptSecuritySBOM
	case CategoryTesting:
		return PromptTesting
	case CategoryResilience:
		return PromptResilience
	}
	// Unknown category → resilience fallback. The renderer also
	// normalises unknown categories, so the two layers agree.
	return PromptResilience
}