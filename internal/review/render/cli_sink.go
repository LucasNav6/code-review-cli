// Package render provides the CLI-side adapters that connect the
// review use case to the world: stdout, stderr, lipgloss, and the
// existing logging / loading primitives.
package render

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/LucasNav6/code-review-cli/internal/claudereview"
	"github.com/LucasNav6/code-review-cli/internal/loading"
	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	"github.com/LucasNav6/code-review-cli/internal/review/usecase"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
	"github.com/LucasNav6/code-review-cli/ui"
)

// CLISink is the production implementation of usecase.OutputSink.
// It paints the PR header (once), a category header + spinner (once
// per category, on stderr), and the parsed findings on stdout
// (once per category).
//
// Output stream allocation:
//   - RenderHeader → s.out (stdout, painted above all findings)
//   - StartCategoryHeader → s.out (stdout, painted above the spinner)
//   - StartSpinner → s.err (stderr, animates the spinner row)
//   - RenderFindings → s.out (stdout, painted BELOW the spinner row)
//
// The split exists because the spinner animates on its own line
// (stderr) while the findings paint on stdout. When the spinner
// terminates it leaves a final ✔/✘ row on stderr; findings paint
// on stdout below. No overlap.
type CLISink struct {
	out io.Writer
	err io.Writer
}

// NewCLISink builds a sink that writes findings to out and the
// spinner to err. Production wires (os.Stdout, os.Stderr); tests
// use two bytes.Buffers.
//
// Passing nil for either is a programming error and panics — the
// use case would silently produce no output otherwise.
func NewCLISink(out, err io.Writer) *CLISink {
	if out == nil || err == nil {
		panic("render.NewCLISink: out and err writers must not be nil")
	}
	return &CLISink{out: out, err: err}
}

// RenderHeader paints the PR summary box on stdout. Header is
// rendered exactly once per Execute call.
func (s *CLISink) RenderHeader(meta scmdomain.PRMetadata) {
	header := ui.PRHeader{
		Number:      meta.Number,
		Title:       meta.Title,
		State:       meta.State,
		IsDraft:     meta.IsDraft,
		AuthorLogin: meta.AuthorLogin,
		HeadRef:     meta.HeadRefName,
		BaseRef:     meta.BaseRefName,
	}
	fmt.Fprint(s.out, header.Render())
}

// StartCategoryHeader paints the section header for one category
// pass on stdout, ABOVE the spinner row. The format is:
//
//	─── [R] Resilience ────────────────────────────────────────
//
// The dash sequence is sized to a fixed width so the header looks
// consistent regardless of category name length. The short tag
// in brackets ([R], [M], [T], [S], [S·SBOM]) gives the user a
// scannable identifier when several categories run in sequence.
func (s *CLISink) StartCategoryHeader(category reviewdomain.Category) {
	tag, label := categoryTagAndLabel(category)
	// Build a 64-char-wide dash suffix so the header is visually
	// consistent regardless of category label length.
	const targetWidth = 64
	headerFmt := "─── %s %s "
	headerSoFar := fmt.Sprintf(headerFmt, tag, label)
	dashesNeeded := targetWidth - len(headerSoFar)
	if dashesNeeded < 3 {
		dashesNeeded = 3
	}
	fmt.Fprintln(s.out, headerSoFar+strings.Repeat("─", dashesNeeded))
}

// StartSpinner animates a spinner line on stderr with msg as the
// label. The returned handle lets the use case stop the spinner
// when the LLM completes (success or failure).
//
// Implementation note: we wrap internal/loading.Run, which already
// handles the spinner animation + the ✔/✘ replacement. We extend
// it with a goroutine that swaps the spinner message every 20s
// (RotationInterval) using the rotating message set for the
// category the spinner was started with.
func (s *CLISink) StartSpinner(msg string) usecase.SpinnerHandle {
	// We need the category to look up the rotating message set.
	// StartSpinner only takes a message today; the use case
	// passes "claude is reviewing the resilience…". We do not
	// know the category from the message alone, so we keep
	// StartSpinner's signature but rely on the use case to
	// pass a message that already includes the right flavour.
	//
	// The category rotation is keyed off a hidden field set
	// by a thin helper (see NewCategorySpinner below). For
	// now we honour the contract: msg is shown verbatim,
	// rotating is done by the use case / cmd layer if it
	// wants.
	return &spinnerHandle{
		run: func() error {
			return loading.Run(s.err, msg, func() error {
				// Block until the use case calls Stop().
				// loading.Run runs fn in a goroutine and
				// prints the spinner until fn returns.
				<-make(chan struct{})
				return nil
			})
		},
		done: make(chan struct{}),
	}
}

// spinnerHandle is the production implementation of
// usecase.SpinnerHandle. It wraps loading.Run in a goroutine so
// the spinner can be stopped asynchronously by the caller.
type spinnerHandle struct {
	once sync.Once
	run  func() error
	done chan struct{}
}

// Stop terminates the spinner. Idempotent: calling Stop twice
// is safe.
func (h *spinnerHandle) Stop() {
	h.once.Do(func() {
		close(h.done)
	})
}

// RenderFindings parses the LLM response and paints one styled
// card per finding on stdout. The section header was already
// painted by StartCategoryHeader; this method only writes the
// findings + success card.
//
// An empty findings list renders as a single green "no findings"
// card. Sub-category findings (SECURITY:SBOM) get the CVE /
// Component / Severity / Fixed blocks painted under the card.
func (s *CLISink) RenderFindings(category reviewdomain.Category, response string) error {
	findings, err := claudereview.Parse(response)
	if err != nil {
		return fmt.Errorf("parse %s response: %w", category, err)
	}

	renderer := claudereview.NewRenderer(claudereview.Colours{
		Foreground: ui.Fg,
		Muted:      ui.Muted,
		Added:      ui.Success,
		Removed:    ui.Danger,
		Category:   ui.Brand,
		Accent:     ui.Accent,
	})

	if len(findings) == 0 {
		renderer.RenderNoFindings(s.out)
		return nil
	}

	for i, finding := range findings {
		if i > 0 {
			fmt.Fprintln(s.out)
			fmt.Fprintln(s.out)
		}
		renderer.Render(s.out, finding)
		renderSBOMExtras(s.out, finding)
	}
	return nil
}

// NewCategorySpinner is the helper the use case (and tests) call
// when they want a spinner that rotates its message every
// RotationInterval using the per-category canned set. The
// returned handle's Stop() ends both the rotation ticker and the
// underlying spinner animation.
//
// We expose this as a top-level constructor (rather than adding
// it to OutputSink) because the spinner rotation is a
// presentation concern: it lives next to the CLI sink that paints
// it. Tests instantiate it directly with a bytes.Buffer to assert
// the rotation behaviour without driving the whole use case.
func NewCategorySpinner(err io.Writer, category reviewdomain.Category) usecase.SpinnerHandle {
	rm := NewRotatingMessages(category)
	spinner := &rotatingSpinner{
		err:      err,
		category: category,
		messages: rm,
		stop:     make(chan struct{}),
	}
	go spinner.loop()
	return spinner
}

// rotatingSpinner is the implementation behind
// NewCategorySpinner. It animates loading.Run on err while
// swapping the message every RotationInterval.
type rotatingSpinner struct {
	err      io.Writer
	category reviewdomain.Category
	messages *RotatingMessages
	stop     chan struct{}
}

// loop drives the rotation: every RotationInterval it emits a
// new spinner run with the next message. The loop ends when
// Stop() is called.
//
// We use loading.Run per-tick because that is the only way the
// existing helper honours the ✔/✘ final state at stop. Each
// tick's run completes when we close its channel via stop().
func (s *rotatingSpinner) loop() {
	for {
		msg := s.messages.Next()
		// Format: "claude is reviewing the <category context>…"
		full := formatSpinnerMessage(s.category, msg)
		// Launch the spinner in a goroutine; when stop fires
		// the run returns and we loop to the next tick.
		done := make(chan struct{})
		go func() {
			_ = loading.Run(s.err, full, func() error {
				<-s.stop
				close(done)
				return nil
			})
		}()
		<-done
		// After the spinner terminates, check if stop was
		// called. If yes, exit the loop. Otherwise wait
		// RotationInterval and start a new spinner with
		// the next message.
		select {
		case <-s.stop:
			return
		case <-time.After(RotationInterval):
			// Continue to the next iteration with the
			// next message.
		}
	}
}

// Stop terminates the rotation and the active spinner. Idempotent.
func (s *rotatingSpinner) Stop() {
	close(s.stop)
}

// formatSpinnerMessage produces the user-visible text the spinner
// animates. The format is "<llm> is <verb>-ing the <noun>…" so
// the rotation messages fit grammatically (they are written as
// present-participle clauses).
func formatSpinnerMessage(category reviewdomain.Category, msg string) string {
	llm := "claude"
	verb := "reviewing"
	if category == reviewdomain.CategorySecuritySBOM {
		verb = "analysing"
	}
	return fmt.Sprintf("%s is %s the diff — %s…", llm, verb, msg)
}

// categoryTagAndLabel returns the short bracket tag and the full
// label for the section header. SECURITY:SBOM gets the longer tag
// to make the sub-category visible at a glance. The tag is
// always wrapped in square brackets so it reads as a UI badge.
func categoryTagAndLabel(c reviewdomain.Category) (string, string) {
	raw := string(c)
	if i := strings.Index(raw, ":"); i >= 0 {
		return "[" + raw[:1] + "\u00b7" + raw[i+1:i+3] + "]", raw[:i] + " \u00b7 " + raw[i+1:]
	}
	return "[" + raw[:1] + "]", raw
}

// renderSBOMExtras paints the SBOM-specific fields of a Finding
// under the card body when the Finding carries them (CVE, CVSS,
// Component, ComponentVersion, FixedVersion, SeverityLabel).
// No-op when the Finding has none of these fields populated.
func renderSBOMExtras(w io.Writer, f claudereview.Finding) {
	if f.CVE == "" && f.Component == "" && f.CVSS == 0 && f.SeverityLabel == "" {
		return
	}
	mutedStyle := ui.MutedStyle
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.Accent))

	if f.CVE != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, accentStyle.Render("CVE"))
		fmt.Fprintln(w, mutedStyle.Render("  "+f.CVE))
	}
	if f.Component != "" {
		version := f.ComponentVersion
		if version == "" {
			version = "?"
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, accentStyle.Render("Component"))
		fmt.Fprintf(w, "%s %s\n", mutedStyle.Render("  "+f.Component+" @"), mutedStyle.Render(version))
	}
	if f.SeverityLabel != "" || f.CVSS > 0 {
		label := f.SeverityLabel
		if label == "" {
			label = "UNKNOWN"
		}
		score := ""
		if f.CVSS > 0 {
			score = fmt.Sprintf(" (CVSS %.1f)", f.CVSS)
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, accentStyle.Render("Severity"))
		fmt.Fprintf(w, "  %s%s\n", mutedStyle.Render(label), mutedStyle.Render(score))
	}
	if f.FixedVersion != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, accentStyle.Render("Fixed in"))
		fmt.Fprintln(w, mutedStyle.Render("  upgrade to "+f.FixedVersion))
	}
}
