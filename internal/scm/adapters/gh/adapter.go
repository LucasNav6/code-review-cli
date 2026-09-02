package gh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LucasNav6/code-review-cli/internal/scm/domain"
	"github.com/LucasNav6/code-review-cli/internal/scm/ports"
)

// ghSCM is the concrete SCM backed by the local `gh` CLI. Construct
// it via New() so callers don't bind to the unexported struct.
type ghSCM struct{}

// New returns a SCM backed by the local `gh` CLI. It is the only
// public constructor — keeping the type unexported means future
// refactors can change the implementation without breaking callers.
func New() ports.SCM {
	return ghSCM{}
}

// Compile-time check: ghSCM implements ports.SCM. If the interface
// ever grows a method, this fails to build instead of panicking at
// runtime.
var _ ports.SCM = ghSCM{}

// ghMetadataResponse is the wire shape from `gh pr view --json`.
// Kept separate from domain.PRMetadata so the nested author object
// (which we collapse to just the login) does not leak into the
// domain.
type ghMetadataResponse struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	State       string `json:"state"`
	IsDraft     bool   `json:"isDraft"`
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
}

// FetchMetadata calls `gh pr view <url> --json ...` and returns the
// parsed domain.PRMetadata. Errors are mapped to scm sentinels via
// the unified classifier (see errors.go) and the standard lib
// context errors.
//
// A successful call that returns a Number==0 or empty Title is
// treated as metadata-fetch failure: gh succeeded but produced
// something the CLI cannot render. We do not log; we surface.
func (ghSCM) FetchMetadata(ctx context.Context, url domain.PRURL) (domain.PRMetadata, error) {
	result, err := runGH(ctx, metadataFetchTimeout,
		"pr", "view", url.String(),
		"--json", "number,title,state,isDraft,headRefName,baseRefName,author",
	)
	if err != nil {
		// Timeout / cancellation propagate as-is; the use case
		// owns context handling.
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return domain.PRMetadata{}, err
		}
		// Everything else is a classified stderr → wrap with the
		// metadata-specific sentinel so the cmd layer can pick the
		// right ErrorType for logging.
		return domain.PRMetadata{}, fmt.Errorf("%w: %v", domain.ErrMetadataFetchFailed, err)
	}

	var raw ghMetadataResponse
	if err := json.Unmarshal(result.Stdout, &raw); err != nil {
		return domain.PRMetadata{}, fmt.Errorf("%w: %v", domain.ErrMetadataFetchFailed, err)
	}

	if raw.Number == 0 || raw.Title == "" {
		return domain.PRMetadata{}, domain.ErrMetadataFetchFailed
	}

	return domain.PRMetadata{
		Number:      raw.Number,
		Title:       raw.Title,
		State:       raw.State,
		IsDraft:     raw.IsDraft,
		HeadRefName: raw.HeadRefName,
		BaseRefName: raw.BaseRefName,
		AuthorLogin: raw.Author.Login,
	}, nil
}

// FetchDiff calls `gh pr diff <url> --color never --patch` and
// returns the raw unified-diff text. Errors flow through the same
// classifier as FetchMetadata; the caller wraps the result with
// ErrDiffFetchFailed so the cmd layer can pick the right ErrorType
// for logging.
//
// The diff is returned as a domain.Diff value (not persisted) so
// the use case can decide whether to feed it straight to the LLM or
// to cache it for re-use.
func (ghSCM) FetchDiff(ctx context.Context, url domain.PRURL) (domain.Diff, error) {
	result, err := runGH(ctx, diffFetchTimeout,
		"pr", "diff", url.String(),
		"--color", "never",
		"--patch",
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return domain.Diff{}, err
		}
		return domain.Diff{}, fmt.Errorf("%w: %v", domain.ErrDiffFetchFailed, err)
	}

	return domain.Diff{Body: result.Stdout}, nil
}