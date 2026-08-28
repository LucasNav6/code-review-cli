package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// cacheTTL define cada cuánto se vuelve a consultar GitHub, para no pegarle
// a la API en cada invocación del comando (y no chocar con el rate limit
// de usuarios anónimos).
const cacheTTL = 24 * time.Hour

type cacheEntry struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

func cachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "code-review", "update-check.json"), nil
}

func readCache() (*cacheEntry, bool) {
	path, err := cachePath()
	if err != nil {
		return nil, false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry cacheEntry

	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	if time.Since(entry.CheckedAt) > cacheTTL {
		return nil, false
	}

	return &entry, true
}

func writeCache(latestVersion string) {
	path, err := cachePath()
	if err != nil {
		return
	}

	entry := cacheEntry{
		CheckedAt:     time.Now(),
		LatestVersion: latestVersion,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}

	_ = os.WriteFile(path, data, 0o644)
}
