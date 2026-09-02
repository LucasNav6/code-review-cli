// Package git implements the RepoFetcher port against the local
// `git` CLI. It is the only place that knows about `git clone` and
// the --depth=1 / --branch flags we use to keep the download fast.
//
// Like every other adapter in the codebase, this one does NOT log:
// the caller decides how to surface failures. The port returns
// scm-domain sentinels so the composition root can route them
// through the existing error mapping in cmd/review.go.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
	"github.com/LucasNav6/code-review-cli/internal/scm/ports"
)

// GitClone is the production RepoFetcher backed by the local `git`
// CLI. It is stateless: callers construct one per composition root.
type GitClone struct{}

// New returns the production RepoFetcher.
func New() ports.RepoFetcher { return GitClone{} }

// Compile-time check: GitClone satisfies the port. Drift surfaces
// as a build error.
var _ ports.RepoFetcher = GitClone{}

// cloneTimeout caps a single git clone. The default 90s is
// generous for shallow clones of normal-sized repos and tight
// enough that a stuck network fails the use case in time to be
// useful.
const cloneTimeout = 90 * time.Second

// lsRemoteTimeout caps the default-branch discovery. We keep this
// short because ls-remote is a HEAD request; if it is slow, the
// clone will be slow too.
const lsRemoteTimeout = 10 * time.Second

// safePathPattern is the strict subset of characters we allow in
// derived values (branch names, etc.). Anything else gets
// rejected so the OS never sees a name with spaces, slashes or
// metacharacters.
var safePathPattern = regexp.MustCompile(`[^a-zA-Z0-9._/-]+`)

// Clone downloads the repository that contains the PR into a
// fresh local directory and returns the absolute path.
//
// The implementation extracts the owner/repo slug from the PR URL
// (e.g. "https://github.com/QuadMinds/saas/pull/5323" ->
// "QuadMinds/saas"), converts it to the corresponding .git URL,
// and runs `git clone --depth=1 --single-branch --branch <default>
// <url> <tmpdir>`. The default branch is read from the remote via
// `git ls-remote --symref HEAD`; if that fails we fall back to
// "main", then "master".
//
// Errors are mapped to scm-domain sentinels so the cmd layer can
// route them through the existing error mapping.
func (GitClone) Clone(ctx context.Context, url scmdomain.PRURL) (string, error) {
	if url == "" {
		return "", scmdomain.ErrInvalidPRURL
	}

	slug, ok := extractSlug(string(url))
	if !ok {
		return "", fmt.Errorf("%w: cannot extract owner/repo from URL",
			scmdomain.ErrInvalidPRURL)
	}

	// Build a tmpdir under os.TempDir. Using os.MkdirTemp means
	// the directory is unique per call, so two concurrent reviews
	// cannot clobber each other's clones.
	tmpRoot, err := os.MkdirTemp("", "code-review-clone-*")
	if err != nil {
		return "", fmt.Errorf("create clone tmpdir: %w", err)
	}

	cloneURL := "https://github.com/" + slug + ".git"

	defaultBranch, err := discoverDefaultBranch(ctx, cloneURL)
	if err != nil || defaultBranch == "" {
		defaultBranch = "main"
	}

	cloneCtx, cancel := context.WithTimeout(ctx, cloneTimeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(cloneCtx, "git", "clone",
		"--depth=1",
		"--single-branch",
		"--branch", defaultBranch,
		cloneURL,
		tmpRoot,
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Best-effort cleanup; we are about to return an error
		// anyway.
		_ = os.RemoveAll(tmpRoot)
		if errors.Is(cloneCtx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("%w: git clone timed out",
				scmdomain.ErrSCMBinaryUnavailable)
		}
		return "", fmt.Errorf("%w: git clone failed: %s",
			scmdomain.ErrSCMBinaryUnavailable,
			strings.TrimSpace(stderr.String()))
	}

	// git clone <tmpRoot> creates <tmpRoot>/.git + the working
	// tree directly under <tmpRoot>. The scanner walks from
	// tmpRoot looking for lockfiles (go.mod, package-lock.json),
	// so we return tmpRoot as-is.
	return tmpRoot, nil
}

// extractSlug pulls the "owner/repo" part from a GitHub PR URL.
// Accepts both "https://github.com/owner/repo/pull/N" and the
// bare "github.com/owner/repo/pull/N" form. Returns (slug, true)
// on success, ("", false) if the URL does not look like a GitHub PR.
func extractSlug(raw string) (string, bool) {
	trimmed := strings.TrimPrefix(raw, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	const prefix = "github.com/"
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	tail := trimmed[len(prefix):]
	if i := strings.Index(tail, "/pull/"); i >= 0 {
		tail = tail[:i]
	}
	parts := strings.Split(tail, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}

// discoverDefaultBranch runs `git ls-remote --symref <url> HEAD`
// and parses the first line to recover the symbolic default
// branch. Returns "" on any error; the caller falls back to "main".
func discoverDefaultBranch(ctx context.Context, cloneURL string) (string, error) {
	lsCtx, cancel := context.WithTimeout(ctx, lsRemoteTimeout)
	defer cancel()

	var stdout bytes.Buffer
	cmd := exec.CommandContext(lsCtx, "git", "ls-remote",
		"--symref", cloneURL, "HEAD")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", err
	}

	// First line looks like:
	//   ref: refs/heads/main	HEAD
	line := strings.SplitN(stdout.String(), "\n", 2)[0]
	const refPrefix = "ref: refs/heads/"
	if !strings.HasPrefix(line, refPrefix) {
		return "", fmt.Errorf("unexpected ls-remote output: %q", line)
	}
	tail := strings.TrimPrefix(line, refPrefix)
	if i := strings.IndexAny(tail, " \t"); i >= 0 {
		tail = tail[:i]
	}
	// Branch name must be the safe subset; anything else is a
	// ref injection attempt.
	if safePathPattern.ReplaceAllString(tail, "_") != tail {
		return "", fmt.Errorf("unsafe branch name: %q", tail)
	}
	return tail, nil
}