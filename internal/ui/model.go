// Package ui implementa la interfaz interactiva (TUI) de code-review con
// Bubble Tea: muestra el progreso de cada etapa de revisión y permite
// navegar los hallazgos o la salida cruda del modelo.
package ui

import (
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/LucasNav6/code-review-cli/internal/claude"
	"github.com/LucasNav6/code-review-cli/internal/githubpr"
	"github.com/LucasNav6/code-review-cli/internal/review"
)

type contentMode int

const (
	modeFindings contentMode = iota
	modeClaude
)

// Model es el estado completo de la TUI.
type Model struct {
	pr     githubpr.PullRequest
	prInfo *githubpr.Info

	diff     string
	diffPath string

	stages []review.Stage

	executingStage int
	selectedStage  int

	mode contentMode

	spinner  spinner.Model
	viewport viewport.Model

	claudeChannel <-chan claude.Event

	width  int
	height int

	loadingText string
	activity    string

	err  error
	done bool
}

// New construye el modelo inicial para revisar el Pull Request dado.
func New(pr githubpr.PullRequest) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(primary)

	vp := viewport.New(
		viewport.WithWidth(70),
		viewport.WithHeight(20),
	)

	return Model{
		pr:             pr,
		diffPath:       "diff_output.txt",
		stages:         review.DefaultStages(),
		executingStage: -1,
		selectedStage:  0,
		mode:           modeFindings,
		spinner:        s,
		viewport:       vp,
		loadingText:    "Buscando la información del pull request...",
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		fetchPRInfoCmd(m.pr),
	)
}

// Err devuelve el error fatal de la ejecución, si lo hubo. Se usa desde la
// capa de CLI para decidir el código de salida del proceso.
func (m Model) Err() error {
	return m.err
}
