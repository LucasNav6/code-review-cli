// Package pr fetches pull-request metadata from the GitHub CLI. It is
// the data source behind the header box the CLI prints once the diff
// is stored.
package pr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/LucasNav6/code-review-cli/helpers"
)

const metadataFetchTimeout = 15 * time.Second

// Metadata is the small subset of pull-request fields the header box
// needs. The JSON tags mirror gh pr view's --json output exactly so
// encoding/json can decode with no intermediate map.
type Metadata struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	State       string `json:"state"`
	IsDraft     bool   `json:"isDraft"`
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
	AuthorLogin string `json:"author"`
}

// ghAuthor mirrors the nested author object gh emits. We only need the
// login (handle) for the header; the full author record stays in gh.
type ghAuthor struct {
	Login string `json:"login"`
}

// ghMetadataResponse is the wire shape from `gh pr view --json`. Keeping
// it separate from Metadata lets us adapt the nested author cleanly.
type ghMetadataResponse struct {
	Number      int      `json:"number"`
	Title       string   `json:"title"`
	State       string   `json:"state"`
	IsDraft     bool     `json:"isDraft"`
	HeadRefName string   `json:"headRefName"`
	BaseRefName string   `json:"baseRefName"`
	Author      ghAuthor `json:"author"`
}

// Fetch calls `gh pr view <url> --json ...` and returns the parsed
// metadata. Errors are returned raw so the caller decides how to log.
func Fetch(ctx context.Context, url string) (Metadata, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return Metadata{}, helpers.ErrEmptyURL
	}

	ctx, cancel := context.WithTimeout(ctx, metadataFetchTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, "gh", "pr", "view", url,
		"--json", "number,title,state,isDraft,headRefName,baseRefName,author")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Metadata{}, helpers.ErrMetadataFetchFailed
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return Metadata{}, categoriseGHError(stderr.String())
		}

		return Metadata{}, helpers.ErrGitHubCLIUnavailable
	}

	var raw ghMetadataResponse
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return Metadata{}, fmt.Errorf("%w: %s", helpers.ErrGitHubCLIInvalid, err)
	}

	if raw.Number == 0 || raw.Title == "" {
		return Metadata{}, helpers.ErrMetadataFetchFailed
	}

	return Metadata{
		Number:      raw.Number,
		Title:       raw.Title,
		State:       raw.State,
		IsDraft:     raw.IsDraft,
		HeadRefName: raw.HeadRefName,
		BaseRefName: raw.BaseRefName,
		AuthorLogin: raw.Author.Login,
	}, nil
}

// categoriseGHError maps stderr from `gh pr view` to a sentinel error.
func categoriseGHError(stderr string) error {
	message := strings.ToLower(stderr)

	switch {
	case strings.Contains(message, "not logged into") ||
		strings.Contains(message, "not authenticated") ||
		strings.Contains(message, "aborted: you are not logged"):
		return helpers.ErrGHNotAuthenticated
	case strings.Contains(message, "no pull requests found") ||
		strings.Contains(message, "could not resolve to a repository") ||
		strings.Contains(message, "not found"):
		return helpers.ErrPRNotFound
	}

	return fmt.Errorf("%w: %s", helpers.ErrMetadataFetchFailed, stderr)
}
