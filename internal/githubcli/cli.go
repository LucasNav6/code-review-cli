package githubcli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/LucasNav6/code-review-cli/helpers"
	"github.com/LucasNav6/code-review-cli/internal/logging"
)

const validationTimeout = 2 * time.Second

func ValidateInstalled(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return logging.LogError(os.Stderr, logging.ErrorTypeGitHubCLINotInstalled, 1, helpers.ErrGitHubCLINotInstalled)
	}

	ctx, cancel := context.WithTimeout(ctx, validationTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "gh", "--version").Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return logging.LogError(os.Stderr, logging.ErrorTypeGitHubCLIUnavailable, 1, helpers.ErrGitHubCLIUnavailable)
		}

		return logging.LogError(os.Stderr, logging.ErrorTypeGitHubCLIUnavailable, 1, helpers.ErrGitHubCLIUnavailable)
	}

	if !strings.HasPrefix(strings.TrimSpace(string(out)), "gh version") {
		return logging.LogError(os.Stderr, logging.ErrorTypeGitHubCLIInvalidOutput, 1, helpers.ErrGitHubCLIInvalid)
	}

	return nil
}
