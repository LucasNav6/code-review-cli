// Package loading provides a single-message CLI spinner powered by
// charm.land/bubbles/v2 + charm.land/lipgloss/v2. It wraps a function call
// so the caller cannot forget to stop the spinner, and replaces the
// spinner with a status glyph (✔ on success, ✘ on failure) so the user
// can see at a glance whether each step succeeded.
package loading

import (
	"fmt"
	"io"
	"os"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
)

const (
	// spinnerColor is the foreground colour used while the spinner is animating.
	spinnerColor = "#8B949E"

	// successColor is the foreground colour of the ✔ glyph emitted on success.
	// Matches the INFO level colour used in internal/logging.
	successColor = "#3FB950"

	// failureColor is the foreground colour of the ✘ glyph emitted on failure.
	// Matches the ERROR level colour used in internal/logging.
	failureColor = "#F85149"

	successGlyph = "✔"
	failureGlyph = "✘"
)

// Run animates a spinner with the given message while fn executes, then
// replaces the spinner line with a status glyph (✔ on success, ✘ on
// failure) and returns fn's error.
//
// If w is nil, os.Stderr is used.
func Run(w io.Writer, message string, fn func() error) error {
	if w == nil {
		w = os.Stderr
	}

	model := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color(spinnerColor))),
	)

	done := make(chan error, 1)

	go func() {
		done <- fn()
	}()

	ticker := time.NewTicker(model.Spinner.FPS)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			printStatus(w, message, err)
			return err
		case <-ticker.C:
			model, _ = model.Update(model.Tick())
			fmt.Fprintf(w, "\r%s %s", model.View(), message)
		}
	}
}

// printStatus rewrites the current line with a coloured status glyph
// followed by the original message. The leading \r returns to column 0
// and \033[2K erases whatever the spinner last wrote so the final line
// is exactly one tidy row.
func printStatus(w io.Writer, message string, err error) {
	glyph := successGlyph
	colour := successColor
	if err != nil {
		glyph = failureGlyph
		colour = failureColor
	}

	style := lipgloss.NewStyle().Foreground(lipgloss.Color(colour))
	fmt.Fprintf(w, "\r\033[2K%s %s\n", style.Render(glyph), message)
}
