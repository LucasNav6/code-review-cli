package render

import (
	"io"

	"github.com/LucasNav6/code-review-cli/internal/logging"
)

// CLINotifier is the production implementation of usecase.Notifier.
// It forwards non-fatal warnings to internal/logging, which already
// owns the format + colour + level semantics.
//
// The notifier does NOT log fatal errors — those propagate through
// Execute's error return and the cmd layer routes them through
// logging.LogError (with the appropriate ErrorType). Mixing the
// two channels would cause double-log.
type CLINotifier struct {
	out io.Writer
}

// NewCLINotifier builds a notifier that writes to w. The writer is
// the same stderr the cmd layer uses for logging so the warnings
// land alongside any other diagnostic output.
func NewCLINotifier(w io.Writer) *CLINotifier {
	if w == nil {
		panic("render.NewCLINotifier: writer must not be nil")
	}
	return &CLINotifier{out: w}
}

// Warn forwards the message to logging.LogWarn. The level is
// chosen to match the documented semantics of "this is non-fatal
// but the user should know": warn is yellow, not red, so the user
// is not alarmed.
func (n *CLINotifier) Warn(msg string) {
	logging.LogWarn(n.out, msg)
}