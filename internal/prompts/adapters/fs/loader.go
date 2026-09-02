// Package fs provides the filesystem-backed implementation of the
// usecase.PromptLoader port. The loader returns the raw template
// body for a given PromptFile; the use case owns the substitution
// of {{DIFF}} so the loader is a dumb read-from-disk.
package fs

import (
	"fmt"
	"os"

	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
)

// Loader reads prompt templates from the local filesystem. The
// path is resolved relative to the process working directory
// (reviewdomain.PromptDir) so the same binary works both when run
// from a checkout (`go run .`) and when installed via the install
// script (which copies the prompts/ folder next to the binary).
type Loader struct{}

// New returns a filesystem-backed Loader. There is no state to
// keep; the function exists so future refactors that need to
// inject configuration can do so without breaking call sites.
func New() *Loader { return &Loader{} }

// Load reads the template body for the given prompt file. The
// prompt's Path() method resolves to "<PromptDir>/<filename>"
// relative to the cwd. Errors are wrapped so the use case can
// route them through its own logging facade.
func (l *Loader) Load(pf reviewdomain.PromptFile) (string, error) {
	data, err := os.ReadFile(pf.Path())
	if err != nil {
		return "", fmt.Errorf("read prompt template %s: %w", pf, err)
	}
	return string(data), nil
}

// Compile-time hint: Loader satisfies usecase.PromptLoader. We do
// not import usecase here (the dependency would point the wrong
// way), but every Loader is wired against a usecase.PromptLoader
// in cmd/review.go. Drift between the two surfaces as a
// build-time error at the call site.