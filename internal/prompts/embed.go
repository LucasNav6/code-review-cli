// Package prompts embebe las plantillas de prompts usadas por cada etapa
// de revisión, para que el binario final sea autocontenido y no dependa de
// archivos externos en el disco del usuario.
package prompts

import _ "embed"

//go:embed templates/security-owasp.md
var Security string

//go:embed templates/maintainability.md
var Maintainability string

//go:embed templates/testing.md
var Testing string

//go:embed templates/resilience.md
var Resilience string
