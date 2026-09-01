package claude

import (
	"context"
	"errors"
	"os/exec"

	"github.com/LucasNav6/code-review-cli/helpers"
)

// ValidateInstalled runs a tiny probe against the local `claude` CLI
// to make sure it is both installed AND authenticated. We cannot
// distinguish "binary missing" from "not logged in" with `exec.LookPath`
// alone, so we issue a real `claude -p "ping"` and inspect the result.
//
// Errors are returned as raw sentinels so the caller decides how to
// route them through the logging facade.
func ValidateInstalled(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, validateTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, "claude", "-p", "ping")

	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return helpers.ErrClaudeNotAuthenticated
		}
		if errors.Is(err, exec.ErrNotFound) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return helpers.ErrClaudeNotInstalled
		}
		return helpers.ErrClaudeUnavailable
	}

	return nil
}
