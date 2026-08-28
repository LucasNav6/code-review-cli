package depscan

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/osv-scanner/v2/pkg/models"
	"github.com/ossf/osv-schema/bindings/go/osvschema"

	"github.com/LucasNav6/code-review-cli/internal/review"
)

// severityRank ordena las categorías de severidad de mayor a menor, para
// poder ordenar los hallazgos.
var severityRank = map[string]int{
	"CRITICAL": 4,
	"HIGH":     3,
	"MEDIUM":   2,
	"LOW":      1,
	"UNKNOWN":  0,
}

// buildResult convierte la salida de osv-scanner en un review.Result, y
// separado arma un resumen legible para mostrar como salida cruda de la
// etapa.
func buildResult(scanRoot string, results models.VulnerabilityResults) (*review.Result, string) {
	var findings []review.Finding

	counts := map[string]int{}
	ecosystems := map[string]bool{}

	for _, source := range results.Results {
		relPath := relativeSourcePath(scanRoot, source.Source.Path)

		for _, pkg := range source.Packages {
			ecosystems[pkg.Package.Ecosystem] = true

			for _, group := range pkg.Groups {
				vuln := representativeVulnerability(pkg.Vulnerabilities, group.IDs)
				if vuln == nil {
					continue
				}

				severity := severityLabel(group.MaxSeverity)
				counts[severity]++

				findings = append(findings, review.Finding{
					File:     relPath,
					Category: severity,
					Title: fmt.Sprintf(
						"%s@%s — %s",
						pkg.Package.Name,
						pkg.Package.Version,
						vuln.GetId(),
					),
					Comment:    vulnerabilitySummary(vuln),
					Suggestion: fixSuggestion(pkg.Package.Name, fixedVersionHint(vuln)),
					Details:    vulnerabilityDetails(vuln, group),
				})
			}
		}
	}

	sortFindingsBySeverity(findings)

	summary := buildSummary(len(findings), counts, ecosystems)

	return &review.Result{
			Summary:  summary,
			Findings: findings,
		},
		renderRawSummary(summary, counts, findings)
}

func representativeVulnerability(vulns []*osvschema.Vulnerability, groupIDs []string) *osvschema.Vulnerability {
	for _, id := range groupIDs {
		for _, vuln := range vulns {
			if vuln.GetId() == id {
				return vuln
			}
		}
	}

	if len(vulns) > 0 {
		return vulns[0]
	}

	return nil
}

func vulnerabilitySummary(vuln *osvschema.Vulnerability) string {
	if summary := strings.TrimSpace(vuln.GetSummary()); summary != "" {
		return summary
	}

	return strings.TrimSpace(vuln.GetDetails())
}

func vulnerabilityDetails(vuln *osvschema.Vulnerability, group models.GroupInfo) []review.Detail {
	var details []review.Detail

	if aliases := otherAliases(vuln.GetId(), group.Aliases); aliases != "" {
		details = append(details, review.Detail{Label: "Alias", Value: aliases})
	}

	if url := firstReferenceURL(vuln); url != "" {
		details = append(details, review.Detail{Label: "Referencia", Value: url})
	}

	return details
}

func otherAliases(id string, aliases []string) string {
	var others []string

	for _, alias := range aliases {
		if alias != id {
			others = append(others, alias)
		}
	}

	return strings.Join(others, ", ")
}

func firstReferenceURL(vuln *osvschema.Vulnerability) string {
	for _, ref := range vuln.GetReferences() {
		if url := ref.GetUrl(); url != "" {
			return url
		}
	}

	return ""
}

// fixedVersionHint busca, dentro de los rangos afectados, la primera
// versión en la que se corrigió la vulnerabilidad.
func fixedVersionHint(vuln *osvschema.Vulnerability) string {
	for _, affected := range vuln.GetAffected() {
		for _, r := range affected.GetRanges() {
			for _, event := range r.GetEvents() {
				if fixed := event.GetFixed(); fixed != "" {
					return fixed
				}
			}
		}
	}

	return ""
}

func fixSuggestion(packageName string, fixedVersion string) string {
	if fixedVersion == "" {
		return "Todavía no hay una versión corregida publicada; evaluar una mitigación alternativa."
	}

	return fmt.Sprintf("Actualizar %s a la versión %s o superior.", packageName, fixedVersion)
}

// severityLabel convierte el score CVSS (0-10) que calcula osv-scanner en
// una categoría legible, siguiendo los rangos estándar de CVSS.
func severityLabel(score string) string {
	value, err := strconv.ParseFloat(score, 64)
	if err != nil {
		return "UNKNOWN"
	}

	switch {
	case value >= 9.0:
		return "CRITICAL"
	case value >= 7.0:
		return "HIGH"
	case value >= 4.0:
		return "MEDIUM"
	case value > 0:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

func sortFindingsBySeverity(findings []review.Finding) {
	for i := 1; i < len(findings); i++ {
		for j := i; j > 0 && severityRank[findings[j].Category] > severityRank[findings[j-1].Category]; j-- {
			findings[j], findings[j-1] = findings[j-1], findings[j]
		}
	}
}

func relativeSourcePath(scanRoot string, sourcePath string) string {
	if rel, err := filepath.Rel(scanRoot, sourcePath); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}

	return sourcePath
}

func buildSummary(total int, counts map[string]int, ecosystems map[string]bool) string {
	if total == 0 {
		if len(ecosystems) == 0 {
			return "No encontré lockfiles reconocidos en este repositorio."
		}

		return "No encontré vulnerabilidades conocidas en las dependencias analizadas."
	}

	parts := make([]string, 0, 4)

	for _, severity := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"} {
		if count := counts[severity]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, severity))
		}
	}

	return fmt.Sprintf("Encontré %d vulnerabilidades conocidas: %s.", total, strings.Join(parts, ", "))
}

func renderRawSummary(summary string, counts map[string]int, findings []review.Finding) string {
	var out strings.Builder

	out.WriteString("Escaneo de dependencias (OSV-Scanner)\n\n")
	out.WriteString(summary)
	out.WriteString("\n")

	if len(findings) == 0 {
		return out.String()
	}

	out.WriteString("\n")

	for _, severity := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"} {
		if count := counts[severity]; count > 0 {
			out.WriteString(fmt.Sprintf("%-10s %d\n", severity, count))
		}
	}

	out.WriteString("\n")

	for _, finding := range findings {
		out.WriteString(fmt.Sprintf("[%s] %s (%s)\n", finding.Category, finding.Title, finding.File))
	}

	return out.String()
}
