package review

import (
	"fmt"
	"strings"
)

// RenderMarkdown vuelca un Result a un documento Markdown legible,
// pensado para guardarse como archivo de salida de la etapa.
func RenderMarkdown(name string, result *Result) string {
	var out strings.Builder

	out.WriteString("# ")
	out.WriteString(name)
	out.WriteString("\n\n")

	if result.Summary != "" {
		out.WriteString(result.Summary)
		out.WriteString("\n\n")
	}

	for i, finding := range result.Findings {
		out.WriteString(fmt.Sprintf("## Hallazgo %d\n\n", i+1))

		if finding.File != "" {
			out.WriteString(fmt.Sprintf("**archivo:** %s  \n", finding.File))
		}

		if finding.Line > 0 {
			out.WriteString(fmt.Sprintf("**línea:** %d  \n", finding.Line))
		}

		if finding.Category != "" {
			out.WriteString(fmt.Sprintf("**categoría:** %s  \n", finding.Category))
		}

		if finding.Title != "" {
			out.WriteString(fmt.Sprintf("**hallazgo:** %s  \n", finding.Title))
		}

		for _, detail := range finding.Details {
			if detail.Value == "" {
				continue
			}

			out.WriteString(fmt.Sprintf("**%s:** %s  \n", detail.Label, detail.Value))
		}

		if finding.Comment != "" {
			out.WriteString(fmt.Sprintf("**comentario:** %s  \n", finding.Comment))
		}

		if finding.Suggestion != "" {
			out.WriteString(fmt.Sprintf("**sugerencia:** %s  \n", finding.Suggestion))
		}

		out.WriteString("\n")
	}

	return out.String()
}
