// Package claudereview parses and renders the structured markdown that
// the bundled prompt templates ask claude to emit. The shape is:
//
//	# <category title>
//
//	## Hallazgo N
//
//	**archivo:** path/to/file.ext
//	**línea:** 123
//	**categoría:** RESILIENCE
//	**comportamiento ante fallo:** ...
//	**observabilidad:** ...
//	**comentario:** ...
//
// Each "## Hallazgo" section becomes one Finding. A response of just
// "NO_FINDINGS" parses to an empty slice and is rendered as a success
// indicator by the caller.
package claudereview

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/LucasNav6/code-review-cli/helpers"
)

// Sentinel errors raised by the parser. They are exported so callers
// can categorise failures without string matching.
var (
	ErrNoFindingsSection = errors.New("claude response contained no `## Hallazgo` section")
	ErrMalformedFinding  = errors.New("claude response had a Hallazgo block with no `archivo` field")
	ErrHunkNotFound      = errors.New("no diff hunk covers the line reported by claude")
)

// Finding is one resilience observation emitted by claude. Fields are
// exported so the renderer can address them by name without coupling
// to the markdown shape.
type Finding struct {
	Number              int
	Archivo             string
	Linea               int
	Categoria           string
	ComportamientoFallo string
	Observabilidad      string
	Comentario          string
}

// headingRE matches `## Hallazgo <number>` regardless of surrounding
// whitespace. The header body that follows is read line by line.
var headingRE = regexp.MustCompile(`(?m)^##\s+Hallazgo\s+(\d+)\s*$`)

// fieldRE captures `**name:** value` pairs. The value can span
// multiple lines until the next recognised field or the next heading.
var fieldRE = regexp.MustCompile(`(?m)^\*\*([^*]+):\*\*\s*(.*)$`)

// Parse converts the raw claude response into a list of Finding. It
// returns an empty slice (not an error) when the response is just
// "NO_FINDINGS" — that is a valid review outcome, not a parse failure.
//
// An error is returned only when the response looks like it was meant
// to contain findings but the structure is unrecognisable (no heading
// at all, or a heading with no parseable fields).
func Parse(markdown string) ([]Finding, error) {
	trimmed := strings.TrimSpace(markdown)
	if trimmed == "" || trimmed == "NO_FINDINGS" {
		return nil, nil
	}

	indices := headingRE.FindAllStringSubmatchIndex(trimmed, -1)
	if len(indices) == 0 {
		return nil, helpers.ErrNoFindingsSection
	}

	findings := make([]Finding, 0, len(indices))
	for i, loc := range indices {
		end := len(trimmed)
		if i+1 < len(indices) {
			end = indices[i+1][0]
		}
		section := trimmed[loc[1]:end]

		finding, err := parseSection(section)
		if err != nil {
			return nil, err
		}
		findings = append(findings, finding)
		finding.Number = i + 1
		findings[len(findings)-1] = finding
	}

	return findings, nil
}

// parseSection extracts the fields of a single `## Hallazgo N` block.
func parseSection(section string) (Finding, error) {
	lines := strings.Split(section, "\n")

	var finding Finding
	var pendingField string
	var pendingValue strings.Builder

	flush := func() {
		if pendingField == "" {
			return
		}
		value := strings.TrimSpace(pendingValue.String())
		assignField(&finding, pendingField, value)
		pendingField = ""
		pendingValue.Reset()
	}

	for _, line := range lines[1:] {
		if strings.HasPrefix(strings.TrimSpace(line), "## Hallazgo") {
			continue
		}

		if matches := fieldRE.FindStringSubmatch(line); matches != nil {
			flush()
			pendingField = strings.TrimSpace(matches[1])
			pendingValue.WriteString(matches[2])
			continue
		}

		if pendingField != "" && strings.TrimSpace(line) != "" {
			if pendingValue.Len() > 0 {
				pendingValue.WriteString(" ")
			}
			pendingValue.WriteString(strings.TrimSpace(line))
		}
	}
	flush()

	if finding.Archivo == "" {
		return Finding{}, helpers.ErrMalformedFinding
	}
	return finding, nil
}

// assignField routes a parsed `**name:** value` pair into the
// matching field on f. Unrecognised keys are silently dropped so
// future prompt additions do not break older binaries.
func assignField(f *Finding, name, value string) {
	switch strings.ToLower(name) {
	case "archivo":
		f.Archivo = value
	case "línea", "linea":
		if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			f.Linea = n
		}
	case "categoría", "categoria":
		f.Categoria = value
	case "comportamiento ante fallo":
		f.ComportamientoFallo = value
	case "observabilidad":
		f.Observabilidad = value
	case "comentario":
		f.Comentario = value
	}
}
