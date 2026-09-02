// Package version agrupa toda la lógica relacionada con versiones del
// binario: lectura de la versión local embebida en el binario en
// tiempo de compilación (buildinfo), formateo para mostrar al usuario,
// y (a futuro) comparación con la última release publicada en GitHub.
//
// La separación entre archivos es deliberada:
//
//   - local.go:   fuente de datos. NO conoce UI ni formato.
//   - format.go:  formato del texto. NO conoce de dónde vienen los datos.
//   - print.go:   une datos + formato + estilo. Es la única pieza que
//                 importa ui/ y la única que el resto del programa
//                 debería invocar para imprimir la versión.
//
// Mantener estas capas separadas permite testear local.go y format.go
// sin levantar cobra ni lipgloss, y reemplazar el render (por ejemplo,
// a JSON) sin tocar el dominio.
package version

import (
	"runtime"
	"strings"

	"github.com/LucasNav6/code-review-cli/buildinfo"
)

// Local describe la versión del binario que está corriendo en este
// proceso. Todos los campos son strings ya saneados: nunca están vacíos
// (si el buildinfo original estaba vacío o era "v", se reemplaza por
// "unknown"). Eso le permite a format.go asumir que puede imprimirlos
// sin hacer más nil-checks ni fallbacks.
type Local struct {
	// Version es la versión semántica del binario sin el prefijo "v"
	// (por ejemplo "1.2.3", NO "v1.2.3"). Se le sacó el "v" para que
	// sea consistente con la convención de la mayoría de los CLIs
	// modernos (kubectl, gh, etc.) al imprimir.
	Version string

	// Commit es el hash corto del commit con el que se compiló el binario.
	Commit string

	// Date es la fecha de build en el formato que vino embebido (típicamente
	// RFC3339, pero se respeta lo que el -ldflags haya inyectado).
	Date string

	// GoVersion es la versión del toolchain de Go con el que se compiló
	// el binario, sin el prefijo "go" (por ejemplo "1.23.1", NO "go1.23.1").
	GoVersion string
}

// GetLocal devuelve los metadatos de build del binario actual, ya
// normalizados para ser seguros de imprimir:
//
//   - El prefijo "v" de la versión semántica se remueve.
//   - Cualquier campo vacío se reemplaza por "unknown" (en lugar de
//     quedar como "" y romper el formato).
//
// La función es pura (no toca red, no toca disco, no tiene efectos
// secundarios) salvo por la lectura de runtime.Version(), que es
// trivial y no inyectable. Eso la hace trivial de testear.
func GetLocal() Local {
	return Local{
		Version:   normaliseVersion(buildinfo.Version),
		Commit:    normaliseField(buildinfo.Commit),
		Date:      normaliseField(buildinfo.Date),
		GoVersion: normaliseGoVersion(runtime.Version()),
	}
}

// normaliseVersion le saca el prefijo "v" a un tag semver ("v1.2.3" ->
// "1.2.3"). Si el resultado queda vacío, devuelve "unknown".
func normaliseVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return "unknown"
	}
	return v
}

// normaliseGoVersion le saca el prefijo "go" a runtime.Version()
// ("go1.23.1" -> "1.23.1"). Si el resultado queda vacío (algo que en
// la práctica no pasa porque runtime siempre devuelve algo), devuelve
// "unknown".
func normaliseGoVersion(v string) string {
	v = strings.TrimPrefix(v, "go")
	if v == "" {
		return "unknown"
	}
	return v
}

// normaliseField es el fallback genérico para campos que no necesitan
// transformación más allá de reemplazar vacío por "unknown".
func normaliseField(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
