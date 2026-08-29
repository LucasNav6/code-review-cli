package ui

import (
	"strings"
	"testing"
)

func TestHighlightCodeKeepsVisibleText(t *testing.T) {
	code := `func main() { return "ok" }`
	rendered := highlightCode("main.go", code)

	if stripANSI(rendered) != code {
		t.Fatalf("expected visible code to remain unchanged, got %q", stripANSI(rendered))
	}

	if !strings.Contains(rendered, "\x1b[") {
		t.Fatal("expected highlighted code to contain ANSI styles")
	}
}

func TestHighlightCodeLeavesUnknownExtensionPlain(t *testing.T) {
	code := `plain text`
	rendered := highlightCode("README.unknown", code)

	if rendered != code {
		t.Fatalf("expected unknown extension to remain plain, got %q", rendered)
	}
}
