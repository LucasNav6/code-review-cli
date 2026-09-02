package gh

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

// Validate checks that the local `gh` CLI is installed AND that it
// answers to --version with output that looks like `gh version ...`.
//
// This is intentionally two checks because they catch different
// failure modes:
//
//  1. exec.LookPath("gh") — the binary is on PATH. Without this the
//     rest of the call would fail with a confusing fork/exec error.
//  2. `gh --version` — the binary that was found is actually gh and
//     not, say, a wrapper script that swallowed the real binary.
//
// Validate returns domain sentinels WITHOUT logging. The previous
// implementation in internal/githubcli/cli.go logged via
// logging.LogError AND returned the same sentinel — a double-log
// bug the caller had to dance around. The contract here matches the
// rest of the adapters: sentinel out, caller decides.
func (ghSCM) Validate(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return domain.ErrSCMBinaryMissing
	}

	// LookPath already found gh; the only way the probe fails now
	// is a timeout, a sandbox killing the subprocess, or a wrapper
	// that swallows the binary. All map to "unavailable" from the
	// caller's perspective.
	result, err := runGH(ctx, validateTimeout, "--version")
	if err != nil {
		// If the timeout fired we report the same unavailable
		// sentinel; the caller already has a deadline-aware ctx.
		if errors.Is(err, context.DeadlineExceeded) {
			return domain.ErrSCMBinaryUnavailable
		}
		return domain.ErrSCMBinaryUnavailable
	}

	if !strings.HasPrefix(strings.TrimSpace(string(result.Stdout)), "gh version") {
		return domain.ErrSCMBinaryInvalid
	}

	return nil
}