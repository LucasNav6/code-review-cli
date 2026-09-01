package review

import (
	"os"
	"path/filepath"

	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/logging"
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

func (s FileStore) Save(diff []byte) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeDiffStore, 1, helpers.ErrDiffStoreFailed)
	}

	if err := os.WriteFile(s.path, diff, 0o644); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeDiffStore, 1, helpers.ErrDiffStoreFailed)
	}

	return nil
}
