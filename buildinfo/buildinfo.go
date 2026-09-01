// Package buildinfo centraliza los metadatos de build del binario.
// Version, Commit y Date se sobrescriben en tiempo de compilación vía
// -ldflags, por ejemplo desde GoReleaser:
//
//	go build -ldflags "\
//	  -X github.com/LucasNav6/code-review-cli/buildinfo.Version=v1.2.3 \
//	  -X github.com/LucasNav6/code-review-cli/buildinfo.Commit=abc1234 \
//	  -X github.com/LucasNav6/code-review-cli/buildinfo.Date=2026-08-28T12:00:00Z"
package buildinfo

var (
	// Version es la versión semántica del binario (por ejemplo "v1.2.3").
	// "dev" indica un build local, no publicado como release.
	Version = "dev"

	// Commit es el hash corto del commit con el que se compiló el binario.
	Commit = "none"

	// Date es la fecha de build en formato RFC3339.
	Date = "unknown"

	// Repo es el repositorio de GitHub "owner/name" contra el que se
	// consultan releases para el chequeo de versión y `code-review upgrade`.
	// Con Repo vacío el chequeo de versión y `upgrade` se desactivan de
	// forma silenciosa en lugar de fallar, así que sigue siendo -ldflags
	// overrideable (por ejemplo para un fork).
	Repo = "LucasNav6/code-review-cli"
)

// IsDev indica si el binario corresponde a un build local (no release).
func IsDev() bool {
	return Version == "dev" || Version == ""
}

// UpdatesConfigured indica si hay un repositorio de releases configurado.
func UpdatesConfigured() bool {
	return Repo != ""
}
