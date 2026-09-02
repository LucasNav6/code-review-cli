package version

import (
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
)

// Print es la única función de este paquete que el resto del programa
// debería invocar para mostrar la versión del binario. Encapsula los
// tres pasos que componen "mostrar la versión":
//
//  1. Leer los metadatos del binario (GetLocal).
//  2. Formatearlos como una línea de texto (FormatLine).
//  3. Aplicarles el estilo provisto por el caller y escribirlos al
//     writer provisto por el caller.
//
// Inyectar el writer y el style (en lugar de hardcodear os.Stdout y
// ui.MutedStyle) cumple dos objetivos:
//
//   - Tests: se puede testear contra un bytes.Buffer y contra un style
//     neutro sin montar la pipeline de lipgloss completa.
//   - Reutilización: si mañana otro comando quiere mostrar la versión
//     con un estilo distinto, no necesita reimplementar la cadena —
//     sólo pasa el style que quiere.
//
// El writer y el style son parámetros explícitos en vez de un struct
// de opciones porque hoy sólo son dos y agregar dependencias a medida
// que aparezcan es mejor que pre-construir una config que después
// queda con un solo campo usado.
func Print(w io.Writer, style lipgloss.Style) error {
	if w == nil {
		// No usamos os.Exit ni panic: la convención en este CLI es
		// devolver error y dejar que Execute() lo loguee. Esto protege
		// contra un caller que accidentalmente pase nil (típico bug
		// cuando se refactorea de cmd.OutOrStdout a otra cosa).
		return fmt.Errorf("version.Print: writer is nil")
	}

	line := FormatLine(GetLocal())
	_, err := fmt.Fprintln(w, style.Render(line))
	return err
}
