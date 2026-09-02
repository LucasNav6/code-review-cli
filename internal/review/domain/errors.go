// Package domain holds the pure types, constants and business rules of
// the review bounded context. It has zero dependencies on UI, cobra,
// subprocess, filesystem, or concrete adapters — everything that
// touches the outside world lives in ../ports/ and ../usecase/.
//
// The package is the source of truth for:
//   - the four review categories (Category),
//   - the values the user can pass via --type (ReviewType),
//   - the prompt filename associated with each category (PromptFile),
//   - the sentinel errors raised inside this bounded context.
//
// Anything that is a constant string the LLM emits in its JSON
// response lives in internal/claudereview (the parser); this package
// only owns the routing decisions that live between the user and the
// LLM.
package domain

import "errors"

// Sentinel errors raised inside the review bounded context. They are
// declared here (and not in helpers/) because they are part of the
// domain contract: a future move to a different binary should be able
// to import internal/review/domain and get every error it can produce
// without depending on the helpers package.
//
// Cross-cutting sentinels that are not specific to the review context
// (e.g. ErrEmptyURL, ErrClaude*) remain in helpers/.
var (
	// ErrUnknownReviewType is returned when a caller hands
	// ResolveCategories a value that is not one of the documented
	// ReviewType values AND is non-empty. An empty string is the
	// documented "use the default" case and never produces this
	// error.
	ErrUnknownReviewType = errors.New("unknown review type")

	// ErrUnknownPromptFile is returned by Category.FromPromptFile
	// when the bare filename does not match any known prompt. It is
	// exported so callers can distinguish "we don't know how to
	// route this prompt" from "the prompt file is missing on disk".
	// The latter is a filesystem concern and lives in the loader
	// adapter.
	ErrUnknownPromptFile = errors.New("unknown prompt file")
)