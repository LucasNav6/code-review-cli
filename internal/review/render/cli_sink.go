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
	"strings"

	"charm.land/lipgloss/v2"

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
//
// Sub-categories (e.g. SECURITY:SBOM) are rendered as a secondary
// label next to the parent category so the user sees both at a
// glance: "─── Claude review (SECURITY · SBOM) ───".
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

	fmt.Fprintf(s.out, "─── Claude review (%s) ───\n", categoryLabel(category))

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

// categoryLabel renders a Category plus its optional Subcategory
// into the section header text. SECURITY:SBOM becomes
// "SECURITY · SBOM". Plain categories render as just "SECURITY".
//
// The dot-separator is U+00B7 (MIDDLE DOT), chosen for visual
// separation without the noise of a slash or a colon.
func categoryLabel(c reviewdomain.Category) string {
	// The Category value can be either the parent ("SECURITY") or
	// the compound form ("SECURITY:SBOM") depending on how the
	// prompt was written. We normalise by reading the Subcategory
	// from the parsed Finding field, but RenderReviewBlock is
	// invoked before the findings are parsed, so we instead sniff
	// the Category string itself.
	raw := string(c)
	if i := strings.Index(raw, ":"); i >= 0 {
		return raw[:i] + " \u00b7 " + raw[i+1:]
	}
	return raw
}

// renderSBOMExtras paints the SBOM-specific fields of a Finding
// under the card body when the Finding carries them (CVE, CVSS,
// Component, ComponentVersion, FixedVersion, SeverityLabel).
// The function is a no-op when the Finding has none of these
// fields populated, so it is safe to call for every finding.
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