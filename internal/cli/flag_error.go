package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

// Detecta los mensajes que pflag genera cuando encuentra
// un flag largo o corto desconocido.
var (
	unknownFlagPattern = regexp.MustCompile(
		`^unknown flag: --(.+)$`,
	)

	unknownShorthandPattern = regexp.MustCompile(
		`^unknown shorthand flag: '.' in -(.+)$`,
	)
)

// Distancia máxima permitida para considerar que un flag
// existente probablemente sea lo que el usuario quiso escribir.
const maxSuggestionDistance = 2

// flagErrorFunc reemplaza el error estándar de Cobra para flags desconocidos.
//
// Si puede identificar un typo y encuentra un flag suficientemente parecido,
// muestra una sugerencia.
func flagErrorFunc(cmd *cobra.Command, err error) error {
	typo, ok := extractUnknownFlag(err)
	if !ok {
		return err
	}

	suggestion := closestFlagName(cmd, typo)

	errorMessage := errorStyle.Render(
		"Unknown flag: --" + typo,
	)

	if suggestion == "" {
		return fmt.Errorf("%s", errorMessage)
	}

	suggestionMessage :=
		mutedStyle.Render("  Did you mean ") +
			updateStyle.Render("`--"+suggestion+"`") +
			mutedStyle.Render("?")

	return fmt.Errorf(
		"%s\n\n%s",
		errorMessage,
		suggestionMessage,
	)
}

// extractUnknownFlag intenta obtener el nombre del flag incorrecto
// a partir del mensaje generado por pflag.
func extractUnknownFlag(err error) (string, bool) {
	message := err.Error()

	if match := unknownFlagPattern.FindStringSubmatch(message); match != nil {
		return match[1], true
	}

	if match := unknownShorthandPattern.FindStringSubmatch(message); match != nil {
		return match[1], true
	}

	return "", false
}

// closestFlagName busca entre los flags visibles del comando
// cuál tiene menor distancia respecto del texto ingresado.
//
// Si ninguno está suficientemente cerca, devuelve string vacío
// para evitar sugerencias incorrectas.
func closestFlagName(cmd *cobra.Command, typo string) string {
	best := ""
	bestDistance := maxSuggestionDistance + 1

	cmd.Flags().VisitAll(func(f *flag.Flag) {
		if f.Hidden {
			return
		}

		distance := levenshteinDistance(typo, f.Name)

		if distance < bestDistance {
			bestDistance = distance
			best = f.Name
		}
	})

	if bestDistance > maxSuggestionDistance {
		return ""
	}

	return best
}

// levenshteinDistance calcula cuántas operaciones hacen falta para
// transformar un string en otro.
func levenshteinDistance(a, b string) int {
	a = strings.ToLower(a)
	b = strings.ToLower(b)

	distances := make([][]int, len(a)+1)

	for i := range distances {
		distances[i] = make([]int, len(b)+1)
		distances[i][0] = i
	}

	for j := range distances[0] {
		distances[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				distances[i][j] = distances[i-1][j-1]
				continue
			}

			deleteCost := distances[i-1][j]
			insertCost := distances[i][j-1]
			replaceCost := distances[i-1][j-1]

			distances[i][j] = min(
				deleteCost,
				insertCost,
				replaceCost,
			) + 1
		}
	}

	return distances[len(a)][len(b)]
}
