// Package fs provides the filesystem-backed implementation of the
// usecase.PromptLoader port. The loader is a dumb read-from-disk:
// it does NOT substitute placeholders. The use case owns the
// substitution policy so the loader stays free of business logic
// and a future in-memory loader (for tests) needs no I/O code.
package fs

import (
	"fmt"
	"os"

	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	"github.com/LucasNav6/code-review-cli/internal/review/usecase"
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
// ctx parameter is accepted for port compatibility but unused:
// the loader is a dumb reader and the use case performs every
// substitution (including {{DIFF}} and {{SBOM}}). The ctx is kept
// in the signature so future loaders (HTTP, embedded, …) can
// inspect it without breaking the contract.
func (l *Loader) Load(pf reviewdomain.PromptFile, _ usecase.PromptContext) (string, error) {
	data, err := os.ReadFile(pf.Path())
	if err != nil {
		return "", fmt.Errorf("read prompt template %s: %w", pf, err)
	}
	return string(data), nil
}