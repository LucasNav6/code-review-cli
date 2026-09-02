package render_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/LucasNav6/code-review-cli/internal/review/render"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// TestNewCLINotifier_NilWriterPanics mirrors the sink contract.
func TestNewCLINotifier_NilWriterPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil writer, got none")
		}
	}()
	render.NewCLINotifier(nil)
}

// TestCLINotifier_Warn_EmitsWarnLevel pins down the format. We
// strip ANSI sequences before asserting so the test is stable
// across lipgloss changes. The level marker is the part the
// caller will visually scan for, so it must be present.
func TestCLINotifier_Warn_EmitsWarnLevel(t *testing.T) {
	var buf bytes.Buffer
	notifier := render.NewCLINotifier(&buf)

	notifier.Warn("category RESILIENCE skipped: claude not installed")

	out := ansiRE.ReplaceAllString(buf.String(), "")
	if !strings.Contains(out, "WARN") {
		t.Errorf("expected WARN level in output, got:\n%s", out)
	}
	if !strings.Contains(out, "category RESILIENCE skipped") {
		t.Errorf("warning message missing from output, got:\n%s", out)
	}
}

// TestCLINotifier_Warn_PreservesCallerMessage locks down that the
// notifier does NOT massage the message. Tests of the use case
// match on substrings in the warning text, so any wrapping would
// break them.
func TestCLINotifier_Warn_PreservesCallerMessage(t *testing.T) {
	var buf bytes.Buffer
	notifier := render.NewCLINotifier(&buf)

	notifier.Warn("hello world")

	out := ansiRE.ReplaceAllString(buf.String(), "")
	if !strings.Contains(out, "hello world") {
		t.Errorf("caller message not preserved, got:\n%s", out)
	}
}