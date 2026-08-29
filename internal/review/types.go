// Package review contiene el dominio de una revisión de código: las etapas
// que se ejecutan, el resultado estructurado que produce cada una y la
// lógica para interpretar y volcar a Markdown la respuesta del modelo.
package review

// Result es la salida estructurada de una etapa de revisión.
type Result struct {
	Summary  string    `json:"summary"`
	Findings []Finding `json:"findings"`
}

// Finding es una observación puntual sobre el diff revisado.
type Finding struct {
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Category   string   `json:"category"`
	Title      string   `json:"title"`
	Comment    string   `json:"comment"`
	Suggestion string   `json:"suggestion,omitempty"`
	Details    []Detail `json:"details,omitempty"`
}

// Detail es un par etiqueta/valor adicional dentro de un Finding
// (por ejemplo "Tests faltantes", "OWASP", etc).
type Detail struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Status representa el estado de ejecución de una etapa de revisión.
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusClean
	StatusFindings
	StatusError
)

// Kind distingue cómo se ejecuta una etapa.
type Kind int

const (
	// KindPrompt es una etapa que le manda un prompt a Claude y parsea su
	// respuesta (seguridad OWASP, mantenibilidad, testing, resiliencia).
	KindPrompt Kind = iota

	// KindCommand es una etapa aislada que corre un chequeo determinístico
	// por comandos/librerías, sin pasar por un LLM (por ejemplo, el escaneo
	// de vulnerabilidades de dependencias con OSV-Scanner).
	KindCommand
)

// Stage es una dimensión de revisión (seguridad, mantenibilidad, etc.)
// junto con su prompt, su salida y su estado de ejecución.
type Stage struct {
	Name       string
	ShortName  string
	Kind       Kind
	Prompt     string
	OutputPath string

	Status Status

	RawOutput string
	Result    *Result
}

// Findings devuelve los hallazgos de la etapa, o nil si todavía no corrió.
func (s Stage) Findings() []Finding {
	if s.Result == nil {
		return nil
	}

	return s.Result.Findings
}

// DefaultStages define el pipeline estándar de revisión.
func DefaultStages() []Stage {
	return []Stage{
		{
			Name:       "Security OWASP",
			ShortName:  "SECURITY",
			Prompt:     securityPrompt(),
			OutputPath: "seguridad.md",
			Status:     StatusPending,
		},
		{
			Name:       "Dependencies",
			ShortName:  "DEPENDENCIES",
			Kind:       KindCommand,
			OutputPath: "dependencias.md",
			Status:     StatusPending,
		},
		{
			Name:       "Readability",
			ShortName:  "READABILITY",
			Prompt:     maintainabilityPrompt(),
			OutputPath: "mantenibilidad.md",
			Status:     StatusPending,
		},
		{
			Name:       "Reliability",
			ShortName:  "RELIABILITY",
			Prompt:     testingPrompt(),
			OutputPath: "testing.md",
			Status:     StatusPending,
		},
		{
			Name:       "Resilience",
			ShortName:  "RESILIENCE",
			Prompt:     resiliencePrompt(),
			OutputPath: "resilience.md",
			Status:     StatusPending,
		},
	}
}
