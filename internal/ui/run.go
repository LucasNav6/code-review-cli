package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/LucasNav6/code-review-cli/internal/githubpr"
)

// Run levanta la interfaz interactiva para revisar el Pull Request dado y
// bloquea hasta que el usuario sale. Devuelve el error fatal ocurrido
// durante la revisión, si lo hubo (por ejemplo, no poder obtener el diff).
func Run(pr githubpr.PullRequest) error {
	m := New(pr)

	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if final, ok := finalModel.(Model); ok {
		return final.Err()
	}

	return nil
}
