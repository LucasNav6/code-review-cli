// Package ports declares the interfaces the scm bounded context
// exposes for consumption from outside (cmd, use cases, scanners).
//
// Two interfaces today:
//
//   - SCM (scm.go): the contract for fetching metadata + diff +
//     validating the binary. The review use case consumes this.
//   - RepoFetcher (repo.go): the contract for cloning a repo to a
//     local directory so external scanners (SBOM, secrets) have
//     filesystem access. Different contract, different port.
//
// Keeping them separate means the review use case does not need to
// pull in git just because scanners do.
package ports

import (
	"context"

	"github.com/LucasNav6/code-review-cli/internal/scm/domain"
)

// RepoFetcher clones a remote git repository to a local directory
// so external tools (osv-scanner, gitleaks, sonarqube) can walk
// the filesystem. The interface is intentionally small:
//
//   - one method (Clone),
//   - one return value (a usable local path),
//   - the same context-cancellation contract every port uses.
//
// Today the only implementation is GitClone (this package,
// internal/scm/adapters/git). Tomorrow an HTTPSDownloadFetcher
// can satisfy the same port for repos that do not expose git.
type RepoFetcher interface {
	// Clone downloads the repository that contains the PR
	// referenced by url into a fresh local directory and returns
	// its absolute path. The directory is created in os.TempDir
	// and the caller is expected to remove it when done (the
	// composition root does this via a defer).
	//
	// The clone is shallow by default (depth=1): we only need the
	// files to feed the scanners, not the history. A future
	// enhancement can add a flag for full clones if a scanner
	// needs them.
	Clone(ctx context.Context, url domain.PRURL) (string, error)
}