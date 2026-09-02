package gh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// diffFetchTimeout caps a single `gh pr diff` call. Long enough for
// big PRs over slow connections, short enough that a hung subprocess
// is reported within the same minute the user is staring at a
// spinner.
const diffFetchTimeout = 30 * time.Second

// metadataFetchTimeout caps a single `gh pr view` call. Metadata is
// small and the call is cheap, so the cap is tighter than the diff
// cap.
const metadataFetchTimeout = 15 * time.Second

// runGHResult is what runGH returns: raw stdout/stderr so the caller
// can decide whether to JSON-decode (metadata) or treat as text
// (diff). Errors are already classified into gh stderr categories
// via classifyStderr.
type runGHResult struct {
	Stdout []byte
	Stderr []byte
}

// runGH is the single exec wrapper every gh call goes through. It
//  1. sets the timeout,
//  2. runs the command with stdout/stderr captured,
//  3. classifies any non-zero exit into a domain sentinel.
//
// runGH does NOT log — the port contract (see /internal/scm/ports)
// is that adapters return raw sentinels and the caller decides how
// to surface them. Logging here would double-log once the use case
// wraps the error in a friendly message.
func runGH(ctx context.Context, timeout time.Duration, args ...string) (runGHResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		// Context cancelled or deadline exceeded: the gh process
		// is already gone (exec.CommandContext kills it), so we
		// surface a context error rather than classify stderr that
		// may not have been written yet.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return runGHResult{}, context.DeadlineExceeded
		}

		// Exit error: gh ran but failed. Map stderr to a sentinel
		// via the unified classifier.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return runGHResult{}, classifyStderr(stderr.String())
		}

		// Any other error (e.g. fork/exec failed because gh was
		// removed between LookPath and Run): bubble up as a
		// formatted error so the caller can wrap it.
		return runGHResult{}, fmt.Errorf("gh: %w", err)
	}

	return runGHResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, nil
}

// validateTimeout is the budget for the `gh --version` probe used by
// Validate. The probe is trivial so the cap is short.
const validateTimeout = 2 * time.Second