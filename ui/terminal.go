package ui

import (
	"os"

	xterm "github.com/charmbracelet/x/term"
)

// defaultTerminalWidth is the fallback used when stdout is not a TTY
// or when the underlying syscall fails (CI, piped output, redirect).
// 120 is a comfortable default that matches the historical 80/132
// convention while leaving room for typical PR headers.
const defaultTerminalWidth = 120

// TerminalWidth returns the width of stdout in cells. The value is
// used to size every fixed-width card the CLI renders so they never
// overflow or look out of place on a wide terminal.
//
// When stdout is not a TTY the call returns defaultTerminalWidth so
// piped output (e.g. `code-review review | tee log.txt`) still
// produces sensible cards.
func TerminalWidth() int {
	w, _, err := xterm.GetSize(os.Stdout.Fd())
	if err != nil || w <= 0 {
		return defaultTerminalWidth
	}
	return w
}
