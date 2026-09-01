package review

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/githubcli"
	"github.com/LucasNav6/code-review-cli/internal/logging"
)

const diffFetchTimeout = 30 * time.Second

func ValidateGitHubCLI(ctx context.Context) error {
	return githubcli.ValidateInstalled(ctx)
}

func StoreDiff(ctx context.Context, diffStore Store, url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return logging.LogError(os.Stderr, logging.ErrorTypeInput, 1, helpers.ErrEmptyURL)
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
			return logging.LogError(os.Stderr, logging.ErrorTypeDiffFetch, 1, helpers.ErrDiffFetchFailed)
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return categoriseGHError(stderr.String())
		}

		return logging.LogError(os.Stderr, logging.ErrorTypeGitHubCLIUnavailable, 1, helpers.ErrGitHubCLIUnavailable)
	}

	return diffStore.Save(stdout.Bytes())
}

func categoriseGHError(stderr string) error {
	message := strings.ToLower(stderr)

	switch {
	case strings.Contains(message, "not logged into") ||
		strings.Contains(message, "not authenticated") ||
		strings.Contains(message, "aborted: you are not logged"):
		return logging.LogError(os.Stderr, logging.ErrorTypeGitHubAuth, 1, helpers.ErrGHNotAuthenticated)
	case strings.Contains(message, "no pull requests found") ||
		strings.Contains(message, "could not resolve to a repository") ||
		strings.Contains(message, "not found"):
		return logging.LogError(os.Stderr, logging.ErrorTypeGitHubPullRequest, 1, helpers.ErrPRNotFound)
	}

	return logging.LogError(os.Stderr, logging.ErrorTypeDiffFetch, 1, helpers.ErrDiffFetchFailed)
}
