// Package loading provides a single-message CLI spinner powered by
// charm.land/bubbles/v2 + charm.land/lipgloss/v2. It wraps a function call
// so the caller cannot forget to stop the spinner.
package loading

import (
	"fmt"
	"io"
	"os"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
)

// spinnerColor is the foreground colour used while the spinner is animating.
// Keeping it as a constant makes the theme easy to swap.
const spinnerColor = "#8B949E"

// Run animates a spinner with the given message while fn executes, then
// stops the spinner and returns fn's error. The spinner is always cleared
// from the output on return, even if fn panics.
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
			fmt.Fprint(w, "\r\033[2K")
			return err
		case <-ticker.C:
			model, _ = model.Update(model.Tick())
			fmt.Fprintf(w, "\r%s %s", model.View(), message)
		}
	}
}
