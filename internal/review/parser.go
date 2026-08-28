package review

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParseResult interpreta la respuesta cruda del modelo para una etapa.
// Tolera varios formatos (JSON, JSON embebido en texto, Markdown y el
// centinela histórico "NO_FINDINGS") porque el modelo no siempre responde
// de forma perfectamente estructurada.
func ParseResult(raw string, defaultCategory string) (*Result, error) {
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return nil, fmt.Errorf("respuesta vacía")
	}

	if isNoFindingsSentinel(raw) {
		return &Result{
			Summary:  "No encontré observaciones relevantes.",
			Findings: []Finding{},
		}, nil
	}

	if result, ok := tryParseJSON(cleanJSONResponse(raw)); ok {
		return result, nil
	}

	if extracted := extractJSONObject(raw); extracted != "" {
		if result, ok := tryParseJSON(extracted); ok {
			return result, nil
		}
	}

	if looksLikeMarkdownReview(raw) {
		result := parseMarkdownReview(raw, defaultCategory)

		if len(result.Findings) > 0 {
			return result, nil
		}
	}

	return nil, fmt.Errorf("no pude reconocer el formato de respuesta de Claude")
}

// FallbackResult produce un hallazgo "de emergencia" que conserva la
// respuesta cruda cuando no pudo interpretarse en ningún formato conocido,
// para no perder la información y poder inspeccionarla manualmente.
func FallbackResult(raw string, category string) *Result {
	return &Result{
		Summary: "Claude devolvió una respuesta que no pude estructurar completamente, pero la conservé para revisión.",
		Findings: []Finding{
			{
				Category: category,
				Title:    "Revisar respuesta de Claude",
				Comment:  "No pude interpretar automáticamente el formato de esta observación. Podés ver la respuesta completa usando Tab.",
				Details: []Detail{
					{
						Label: "Respuesta recibida",
						Value: truncate(strings.TrimSpace(raw), 500),
					},
				},
			},
		},
	}
}

// =============================================================================
// Sentinela "sin hallazgos"
// =============================================================================

func isNoFindingsSentinel(raw string) bool {
	normalized := strings.TrimSpace(strings.ToUpper(raw))

	switch normalized {
	case "NO_FINDINGS", "NO FINDINGS", "NO_FINDING":
		return true
	default:
		return false
	}
}

// =============================================================================
// JSON
// =============================================================================

func tryParseJSON(candidate string) (*Result, bool) {
	var result Result

	if err := json.Unmarshal([]byte(candidate), &result); err != nil {
		return nil, false
	}

	if result.Findings == nil {
		result.Findings = []Finding{}
	}

	return &result, true
}

func cleanJSONResponse(raw string) string {
	value := strings.TrimSpace(raw)

	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```JSON")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")

	return strings.TrimSpace(value)
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")

	if start == -1 || end == -1 || end <= start {
		return ""
	}

	return raw[start : end+1]
}

// =============================================================================
// Markdown
// =============================================================================

func looksLikeMarkdownReview(raw string) bool {
	return strings.Contains(raw, "## Hallazgo") ||
		strings.Contains(raw, "**archivo:**") ||
		strings.Contains(raw, "**Archivo:**")
}

func parseMarkdownReview(raw string, defaultCategory string) *Result {
	result := &Result{
		Summary:  "",
		Findings: []Finding{},
	}

	sections := splitMarkdownFindings(raw)

	if len(sections) == 0 {
		return result
	}

	for _, section := range sections {
		finding := parseMarkdownFinding(section, defaultCategory)

		if isUsefulFinding(finding) {
			result.Findings = append(result.Findings, finding)
		}
	}

	if len(result.Findings) > 0 {
		result.Summary = fmt.Sprintf(
			"Claude encontró %d %s para revisar.",
			len(result.Findings),
			plural(len(result.Findings), "observación", "observaciones"),
		)
	}

	return result
}

var findingHeadingRe = regexp.MustCompile(`(?m)^##\s+Hallazgo\s+\d+`)

func splitMarkdownFindings(raw string) []string {
	locations := findingHeadingRe.FindAllStringIndex(raw, -1)

	if len(locations) == 0 {
		// En algunos casos el modelo puede devolver un solo hallazgo sin heading.
		if strings.Contains(strings.ToLower(raw), "**archivo:**") {
			return []string{raw}
		}

		return nil
	}

	sections := make([]string, 0, len(locations))

	for i, location := range locations {
		start := location[0]
		end := len(raw)

		if i+1 < len(locations) {
			end = locations[i+1][0]
		}

		sections = append(sections, strings.TrimSpace(raw[start:end]))
	}

	return sections
}

func parseMarkdownFinding(section string, defaultCategory string) Finding {
	finding := Finding{
		Category: defaultCategory,
		Details:  []Detail{},
	}

	lines := strings.Split(section, "\n")

	var currentField string
	var currentValue strings.Builder

	flush := func() {
		value := strings.TrimSpace(currentValue.String())

		if currentField == "" || value == "" {
			currentField = ""
			currentValue.Reset()
			return
		}

		assignMarkdownField(&finding, currentField, value)

		currentField = ""
		currentValue.Reset()
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)

		if line == "" {
			if currentField != "" {
				currentValue.WriteString("\n")
			}

			continue
		}

		if strings.HasPrefix(line, "## Hallazgo") {
			continue
		}

		field, value, ok := parseMarkdownFieldLine(line)

		if ok {
			flush()

			currentField = field
			currentValue.WriteString(value)

			continue
		}

		// Continuación multilínea del campo actual.
		if currentField != "" {
			if currentValue.Len() > 0 {
				currentValue.WriteString(" ")
			}

			currentValue.WriteString(line)
		}
	}

	flush()

	if finding.Title == "" && finding.Comment != "" {
		finding.Title = shortTitleFromComment(finding.Comment)
	}

	return finding
}

var markdownFieldLineRe = regexp.MustCompile(`^\*\*([^*]+):\*\*\s*(.*)$`)

func parseMarkdownFieldLine(line string) (field string, value string, ok bool) {
	match := markdownFieldLineRe.FindStringSubmatch(line)

	if len(match) != 3 {
		return "", "", false
	}

	return strings.TrimSpace(match[1]), strings.TrimSpace(match[2]), true
}

func assignMarkdownField(finding *Finding, field string, value string) {
	normalized := strings.ToLower(strings.TrimSpace(field))

	switch normalized {
	case "archivo":
		finding.File = value

	case "línea", "linea":
		finding.Line = parseLineNumber(value)

	case "categoría", "categoria":
		finding.Category = value

	case "hallazgo":
		finding.Title = value

	case "comentario":
		finding.Comment = value

	case "sugerencia", "sugerencia de refactor":
		finding.Suggestion = value

	case "tests cubiertos":
		finding.Details = append(finding.Details, Detail{Label: "Tests cubiertos", Value: value})

	case "tests faltantes", "tests faltante":
		finding.Details = append(finding.Details, Detail{Label: "Tests faltantes", Value: value})

	case "caso borde":
		finding.Details = append(finding.Details, Detail{Label: "Caso borde", Value: value})

	case "comportamiento ante fallo":
		finding.Details = append(finding.Details, Detail{Label: "Comportamiento ante fallo", Value: value})

	case "observabilidad":
		finding.Details = append(finding.Details, Detail{Label: "Observabilidad", Value: value})

	case "owasp":
		finding.Details = append(finding.Details, Detail{Label: "OWASP", Value: value})

	default:
		finding.Details = append(finding.Details, Detail{Label: field, Value: value})
	}
}

var lineNumberRe = regexp.MustCompile(`\d+`)

func parseLineNumber(value string) int {
	match := lineNumberRe.FindString(value)

	if match == "" {
		return 0
	}

	number, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}

	return number
}

func isUsefulFinding(finding Finding) bool {
	return finding.File != "" ||
		finding.Title != "" ||
		finding.Comment != "" ||
		finding.Suggestion != ""
}

func shortTitleFromComment(comment string) string {
	comment = strings.TrimSpace(comment)

	if comment == "" {
		return "Observación"
	}

	const maxLength = 80

	if len(comment) <= maxLength {
		return comment
	}

	return strings.TrimSpace(comment[:maxLength]) + "..."
}

// =============================================================================
// Helpers genéricos
// =============================================================================

func plural(value int, singular string, pluralForm string) string {
	if value == 1 {
		return singular
	}

	return pluralForm
}

func truncate(value string, length int) string {
	if len(value) <= length {
		return value
	}

	return value[:length] + "..."
}
