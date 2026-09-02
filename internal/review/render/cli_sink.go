// Package render provides the CLI-side adapters that connect the
// review use case to the world: stdout, stderr, lipgloss, and the
// existing logging / loading primitives.
//
// The package lives under internal/review/ (not internal/review/
// render/ — wait, it does, by design) because the adapters are
// specific to the review flow. Future bounded contexts that need
// CLI adapters would either share this one or grow their own.
package render

import (
	"fmt"
	"io"

	"github.com/LucasNav6/code-review-cli/internal/claudereview"
	reviewdomain "github.com/LucasNav6/code-review-cli/internal/review/domain"
	scmdomain "github.com/LucasNav6/code-review-cli/internal/scm/domain"
	"github.com/LucasNav6/code-review-cli/ui"
)

// CLISink is the production implementation of usecase.OutputSink.
// It paints the PR header on stdout (once) and one styled card per
// finding per category on stdout (once per category).
//
// All output is written to the writer injected at construction
// time. Tests use a bytes.Buffer; production wires os.Stdout via
// the composition root (H6).
type CLISink struct {
	out io.Writer
}

// NewCLISink builds a sink that writes to w. Passing nil is a
// programming error and panics — the use case would silently
// produce no output otherwise.
func NewCLISink(w io.Writer) *CLISink {
	if w == nil {
		panic("render.NewCLISink: writer must not be nil")
	}
	return &CLISink{out: w}
}

// RenderHeader paints the PR summary box. The header carries the
// title, state badge, author, and branch refs — everything the
// user needs to recognise which PR they are reviewing.
//
// Header is rendered exactly once per Execute call. The use case
// guarantees this; the sink does not need to deduplicate.
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

// RenderReviewBlock parses the LLM response into findings and
// paints one styled card per finding. A parse failure surfaces as
// an error so the use case can warn and continue (the contract
// from usecase.OutputSink says these errors are non-fatal).
//
// An empty findings list renders as a single green "no findings"
// card. A response that is exactly the literal "NO_FINDINGS"
// (after trim + fence stripping, see claudereview.Parse) does the
// same. The category label is used as a section header so the
// user can tell which block corresponds to which --type pass.
func (s *CLISink) RenderReviewBlock(category reviewdomain.Category, response string) error {
	findings, err := claudereview.Parse(response)
	if err != nil {
		// Parse failure is non-fatal — the use case logs the
		// warning. We return the error so the use case can match
		// on it if it wants, but the caller treats it as a warn.
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

	fmt.Fprintf(s.out, "─── Claude review (%s) ───\n", category)

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
	}
	return nil
}