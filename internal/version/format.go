package version

import "fmt"

// FormatLine produce la línea de texto que describe la versión del
// binario, lista para que un consumidor externo la renderice (con o
// sin estilo, a stdout o a un archivo, etc.).
//
// El formato es:
//
//	code-review CLI <version> (commit <commit>, built <date>, Go <go>)
//
// Se eligió mantenerlo en una sola línea para que sea fácil de grepear,
// de copiar y de pegar en un bug report. Si en el futuro hace falta
// más detalle (por ejemplo, el path del binario), se prefiere agregar
// un FormatVerbose separado antes que saturar FormatLine.
//
// Esta función es pura: no tiene efectos secundarios, no toca red, no
// toca disco, no lee del entorno. Sólo toma un Local y devuelve un
// string. Eso permite testearla con tablas de casos sin levantar nada.
func FormatLine(l Local) string {
	return fmt.Sprintf("code-review CLI %s (commit %s, built %s, Go %s)",
		l.Version, l.Commit, l.Date, l.GoVersion)
}
