package ui

import (
	"os/exec"

	tea "charm.land/bubbletea/v2"
)

// preflightState indica en qué punto de la validación de dependencias
// externas está la TUI.
type preflightState int

const (
	preflightRunning preflightState = iota
	preflightPassed
	preflightFailed
)

// dependency es un binario externo que code-review necesita para poder
// funcionar.
type dependency struct {
	Label       string
	Command     string
	InstallHint string
}

var requiredDependencies = []dependency{
	{
		Label:       "GitHub CLI (gh)",
		Command:     "gh",
		InstallHint: "instalalo desde https://cli.github.com y autenticate con \"gh auth login\"",
	},
	{
		Label:       "Claude Code CLI (claude)",
		Command:     "claude",
		InstallHint: "instalalo desde https://docs.claude.com/claude-code",
	},
}

type dependencyResult struct {
	dependency
	Found bool
}

func allDependenciesFound(results []dependencyResult) bool {
	for _, result := range results {
		if !result.Found {
			return false
		}
	}

	return true
}

type preflightMsg struct {
	results []dependencyResult
}

// checkDependenciesCmd valida, antes de pedir nada a GitHub, que el usuario
// tenga instalados los binarios externos que code-review necesita.
func checkDependenciesCmd() tea.Cmd {
	return func() tea.Msg {
		results := make([]dependencyResult, len(requiredDependencies))

		for i, dep := range requiredDependencies {
			_, err := exec.LookPath(dep.Command)

			results[i] = dependencyResult{
				dependency: dep,
				Found:      err == nil,
			}
		}

		return preflightMsg{results: results}
	}
}
