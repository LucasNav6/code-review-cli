package ui

import (
	"strings"

	"github.com/LucasNav6/code-review-cli/internal/review"
)

// section agrupa una o más stages bajo una misma tab visual. Dependencies
// vive dentro de RISK porque, para quien lee el review, ambas responden la
// misma pregunta ("¿este PR introduce riesgo?"): una la contesta un LLM
// leyendo el diff, la otra un scanner de vulnerabilidades conocidas.
//
// El orden y el agrupamiento de stageIndices asume el pipeline fijo de
// review.DefaultStages(): 0=Security, 1=Dependencies, 2=Readability,
// 3=Reliability, 4=Resilience.
type section struct {
	Label        string
	Subtitle     string
	StageIndices []int
}

func sections() []section {
	return []section{
		{
			Label:        "RISK",
			Subtitle:     "Security and dependency risks introduced by this Pull Request.",
			StageIndices: []int{0, 1},
		},
		{
			Label:        "Readability",
			Subtitle:     "Clarity, complexity and maintainability.",
			StageIndices: []int{2},
		},
		{
			Label:        "Reliability",
			Subtitle:     "Correctness, tests and edge cases.",
			StageIndices: []int{3},
		},
		{
			Label:        "Resilience",
			Subtitle:     "Failures, retries, observability and recovery.",
			StageIndices: []int{4},
		},
	}
}

func sectionIndexForStage(stageIndex int) int {
	for i, sec := range sections() {
		for _, idx := range sec.StageIndices {
			if idx == stageIndex {
				return i
			}
		}
	}

	return 0
}

func (m Model) currentSection() section {
	return sections()[sectionIndexForStage(m.selectedStage)]
}

func stagesForSection(allStages []review.Stage, sec section) []review.Stage {
	stages := make([]review.Stage, len(sec.StageIndices))

	for i, idx := range sec.StageIndices {
		stages[i] = allStages[idx]
	}

	return stages
}

func (m Model) currentSectionStages() []review.Stage {
	return stagesForSection(m.stages, m.currentSection())
}

// sectionStatus resume el estado de varias stages en un único status: el
// peor caso manda (un error opaca todo lo demás, seguido de "todavía
// corriendo", etc), así RISK refleja el estado combinado de seguridad y
// dependencias.
func sectionStatus(stages []review.Stage) review.Status {
	var hasRunning, hasFindings, hasClean bool

	for _, stage := range stages {
		switch stage.Status {
		case review.StatusError:
			return review.StatusError
		case review.StatusRunning:
			hasRunning = true
		case review.StatusFindings:
			hasFindings = true
		case review.StatusClean:
			hasClean = true
		}
	}

	switch {
	case hasRunning:
		return review.StatusRunning
	case hasFindings:
		return review.StatusFindings
	case hasClean:
		return review.StatusClean
	default:
		return review.StatusPending
	}
}

func sectionFindings(stages []review.Stage) []review.Finding {
	var findings []review.Finding

	for _, stage := range stages {
		findings = append(findings, stage.Findings()...)
	}

	return findings
}

// sectionRawOutput concatena la salida cruda de todas las stages de la
// sección. Cuando la sección agrupa más de una stage (RISK), antepone el
// nombre de cada una para no perder de dónde viene cada bloque.
func sectionRawOutput(stages []review.Stage) string {
	if len(stages) == 1 {
		return stages[0].RawOutput
	}

	var parts []string

	for _, stage := range stages {
		if stage.RawOutput == "" {
			continue
		}

		parts = append(parts, sectionStyle.Render(stage.Name)+"\n\n"+stage.RawOutput)
	}

	return strings.Join(parts, "\n\n\n")
}
