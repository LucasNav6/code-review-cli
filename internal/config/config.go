// Package config persists user preferences for the code-review CLI.
// Settings live in a JSON file under the OS-recommended config
// directory (os.UserConfigDir), e.g.:
//
//	macOS:   ~/Library/Application Support/code-review/config.json
//	Linux:   ~/.config/code-review/config.json
//	Windows: %AppData%/code-review/config.json
//
// Today the only persisted key is the LLM provider name. Future keys
// (prompt template path, timeout overrides, verbosity) slot into the
// same JSON without a schema migration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/LucasNav6/code-review-cli/helpers"
)

// Config is the on-disk shape of the user's preferences. Fields are
// pointers so we can distinguish "not set" from "set to zero value".
type Config struct {
	// Provider is the name of the LLM backend used for code review.
	// nil means "use the default" (claude). A non-nil empty string
	// is treated as invalid.
	Provider *string `json:"provider,omitempty"`
}

const (
	fileName = "config.json"
	dirName  = "code-review"
	// DefaultProvider is used when the user has not picked one.
	DefaultProvider = "claude"
)

// Store reads and writes the config file. Safe for concurrent use.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore returns a Store pointing at the standard config location.
// Callers should usually want the package-level singleton from
// DefaultStore, but exposing the constructor makes tests easier.
func NewStore() (*Store, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return nil, fmt.Errorf("resolve user config dir: %w", err)
	}
	return &Store{path: filepath.Join(base, dirName, fileName)}, nil
}

// DefaultStore is the singleton used by the CLI. Initialisation is
// lazy so a transient failure to resolve the config dir does not
// crash import time.
var (
	defaultOnce sync.Once
	defaultErr  error
	defaultSt   *Store
)

// DefaultStore returns the process-wide Store, initialised lazily.
// The error is captured on first call and replayed on every later
// call so the CLI can decide how to surface a missing config dir.
func DefaultStore() (*Store, error) {
	defaultOnce.Do(func() {
		defaultSt, defaultErr = NewStore()
	})
	return defaultSt, defaultErr
}

// Path returns the absolute path the Store reads from / writes to.
func (s *Store) Path() string {
	return s.path
}

// Load reads the config file. If the file does not exist it returns
// the zero Config — that is the desired outcome for a fresh install.
func (s *Store) Load() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return c, nil
}

// Save writes the config atomically (temp file + rename) so a crash
// mid-write cannot corrupt the user's preferences.
func (s *Store) Save(c Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp config: %w", err)
	}
	return nil
}

// GetProvider returns the persisted provider name, or DefaultProvider
// if the user has not picked one.
func (s *Store) GetProvider() (string, error) {
	c, err := s.Load()
	if err != nil {
		return "", err
	}
	if c.Provider == nil {
		return DefaultProvider, nil
	}
	return *c.Provider, nil
}

// SetProvider validates and persists the provider name.
func (s *Store) SetProvider(name string) error {
	if name == "" {
		return helpers.ErrInvalidConfigValue
	}
	c, err := s.Load()
	if err != nil {
		return err
	}
	c.Provider = &name
	return s.Save(c)
}

// UnsetProvider clears the persisted provider so the default takes over.
func (s *Store) UnsetProvider() error {
	c, err := s.Load()
	if err != nil {
		return err
	}
	c.Provider = nil
	return s.Save(c)
}

// KnownProvider reports whether name is a recognised provider name.
// Empty input is treated as valid because it maps to the default.
func KnownProvider(name string) bool {
	if name == "" {
		return true
	}
	for _, known := range KnownProviderNames() {
		if name == known {
			return true
		}
	}
	return false
}

// KnownProviderNames is the source of truth for provider identifiers.
// The list mirrors what internal/llm.New can resolve.
func KnownProviderNames() []string {
	return []string{"claude", "codex"}
}
