package domain

import "errors"

// Sentinel errors raised inside the llm bounded context.
//
// We do NOT redeclare helpers.ErrProviderNotImpl / ErrUnknownProvider
// here on purpose: those are still referenced by internal/config
// (which has not been migrated yet). When config moves to the new
// layout, these sentinels will replace the helpers versions and the
// old ones will be deleted.
var (
	// ErrProviderNotImpl is returned by stub providers (e.g. codex
	// today) that the user can select but that have no real
	// implementation yet. The message must mention which provider
	// is unimplemented so the user knows what to fix in their
	// config.
	ErrProviderNotImpl = errors.New("llm: provider is not implemented yet")

	// ErrUnknownProvider is returned when the user asks for a
	// provider name that is not registered. The Resolver port
	// returns this so the caller can present the list of known
	// names in the error message.
	ErrUnknownProvider = errors.New("llm: unknown provider")

	// ErrProviderInvalid is returned when the resolved provider
	// name is recognised but the constructor rejected it (e.g.
	// missing config). Kept separate from ErrUnknownProvider so
	// the cmd layer can pick the right ErrorType for logging.
	ErrProviderInvalid = errors.New("llm: provider is not valid in this environment")
)