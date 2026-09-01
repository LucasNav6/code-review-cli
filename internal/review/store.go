package review

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasNav6/code-review-cli/helpers"
)

const diffFilename = "review.diff"

type Store interface {
	Save(diff []byte) error
}

type FileStore struct {
	path string
}

func NewFileStore() FileStore {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}

	return FileStore{
		path: filepath.Join(dir, "code-review", diffFilename),
	}
}

// Save writes the diff to the cache file. Errors are returned as raw
// sentinels so callers can decide how to surface them — saving the
// log line in this layer would duplicate messages when the caller
// already routes failures through its own logging facade.
func (s FileStore) Save(diff []byte) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", helpers.ErrDiffStoreFailed)
	}

	if err := os.WriteFile(s.path, diff, 0o644); err != nil {
		return fmt.Errorf("write diff: %w", helpers.ErrDiffStoreFailed)
	}

	return nil
}
