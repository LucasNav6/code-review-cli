// Package executil builds exec.Cmd instances that are safe to cancel.
//
// exec.CommandContext's default cancellation only kills the direct child
// process. Some of the CLIs we shell out to (notably `git clone` and
// `git ls-remote`, which fork `git-remote-https` to do the actual network
// transfer) spawn grandchildren that survive that kill, becoming orphans
// reparented to init if the parent Go process exits or its context is
// cancelled before the subprocess finishes on its own.
//
// Command puts every subprocess in its own process group and kills the
// whole group on cancellation, so no grandchild is left running once ctx
// is done.
package executil

import (
	"context"
	"os/exec"
	"time"
)

// killGraceDelay caps how long Wait blocks after cancellation before the
// process is force-killed. See exec.Cmd.WaitDelay.
const killGraceDelay = 5 * time.Second

// Command is a drop-in replacement for exec.CommandContext that also
// arranges for the subprocess's whole process group to be killed when ctx
// is done, instead of just the direct child.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = killGraceDelay
	setProcessGroup(cmd)
	return cmd
}
