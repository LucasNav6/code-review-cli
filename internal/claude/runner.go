// Package claude wraps the local `claude` CLI so the rest of the
// codebase can ask for an LLM review without knowing about the
// subprocess mechanics.
package claude

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/LucasNav6/code-review-cli/helpers"
)

const (
	// runTimeout caps how long a single claude invocation can run.
	// Reviews of large diffs can take a while, so the cap is generous
	// (7.5 minutes). Failures from this timeout propagate to the
	// caller as ErrClaudeTimeout.
	runTimeout = 7*time.Minute + 30*time.Second

	// validateTimeout is shorter because the validation probe is a
	// trivial ping-and-go exchange.
	validateTimeout = 15 * time.Second
)

// Run invokes `claude -p <prompt>` and returns the captured stdout
// (trimmed of leading/trailing whitespace). On any failure it returns
// a sentinel error — logging is the caller's responsibility so we do
// not end up with double-logged messages when the caller also wants
// to surface the failure.
func Run(ctx context.Context, prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", helpers.ErrClaudeInvalid
	}

	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, "claude", "-p", prompt)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", helpers.ErrClaudeTimeout
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", categoriseClaudeError(stderr.String())
		}

		return "", helpers.ErrClaudeUnavailable
	}

	return strings.TrimSpace(stdout.String()), nil
}

// categoriseClaudeError maps the stderr of a failed `claude` call to
// a sentinel error. Callers turn these into user-facing messages via
// their own logging facade.
func categoriseClaudeError(stderr string) error {
	message := strings.ToLower(stderr)

	switch {
	case strings.Contains(message, "not logged in") ||
		strings.Contains(message, "not authenticated") ||
		strings.Contains(message, "please run /login") ||
		strings.Contains(message, "login"):
		return helpers.ErrClaudeNotAuthenticated
	}

	return fmt.Errorf("%w: %s", helpers.ErrClaudeUnavailable, stderr)
}
