package review

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasNav6/code-review-cli/helpers"
)

// LoadStoredDiff reads the diff that StoreDiff wrote to the cache
// directory. It returns the raw bytes so callers can substitute them
// into a prompt template verbatim. Errors are returned as raw
// sentinels so callers control logging.
func LoadStoredDiff() ([]byte, error) {
	store := NewFileStore()

	data, err := os.ReadFile(store.path)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %v", helpers.ErrDiffStoreFailed, store.path, err)
	}

	if len(data) == 0 {
		return nil, helpers.ErrDiffStoreFailed
	}

	return data, nil
}

// BuildPrompt reads the prompt template at templatePath, substitutes
// the {{DIFF}} placeholder once with diff, and returns the resulting
// prompt string ready to be sent to claude.
//
// The templatePath is resolved relative to the current working
// directory so callers can pass paths like "internal/prompts/resilience.md".
func BuildPrompt(templatePath string, diff []byte) (string, error) {
	template, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read prompt template %s: %w", templatePath, err)
	}

	// Single substitution is intentional: the template is plain
	// markdown with exactly one variable slot, so a templating engine
	// would be overkill. If the placeholder is missing, we append the
	// diff at the end so claude still has something to review.
	body := strings.ReplaceAll(string(template), "{{DIFF}}", string(diff))
	if body == string(template) {
		body += "\n\n" + string(diff)
	}

	return body, nil
}

// DiffPath returns the absolute path of the cached diff file. It is
// exposed so cmd/review.go can print it for transparency after the
// review completes.
func DiffPath() string {
	store := NewFileStore()
	return filepath.Clean(store.path)
}
