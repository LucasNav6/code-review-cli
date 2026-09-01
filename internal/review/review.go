package review

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/claude"
	"github.com/LucasNav6/code-review-cli/internal/githubcli"
)

const diffFetchTimeout = 30 * time.Second

// ValidateGitHubCLI checks that the local `gh` CLI is installed.
// It returns a raw sentinel so the caller can route the failure to
// its logging facade.
func ValidateGitHubCLI(ctx context.Context) error {
	return githubcli.ValidateInstalled(ctx)
}

// ValidateClaude checks that the local `claude` CLI is installed and
// authenticated. Thin re-export of internal/claude.
func ValidateClaude(ctx context.Context) error {
	return claude.ValidateInstalled(ctx)
}

// StoreDiff fetches the diff for the given PR URL and persists it
// through diffStore. Errors are returned raw so the caller controls
// logging.
func StoreDiff(ctx context.Context, diffStore Store, url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return helpers.ErrEmptyURL
	}

	ctx, cancel := context.WithTimeout(ctx, diffFetchTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, "gh", "pr", "diff", url, "--color", "never", "--patch")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return helpers.ErrDiffFetchFailed
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return categoriseGHError(stderr.String())
		}

		return helpers.ErrGitHubCLIUnavailable
	}

	return diffStore.Save(stdout.Bytes())
}

// categoriseGHError maps stderr from `gh pr diff` to a sentinel error.
// The mapping is intentionally narrow: callers turn these into user-
// facing error messages via their own logging facade.
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

	return fmt.Errorf("%w: %s", helpers.ErrDiffFetchFailed, stderr)
}
