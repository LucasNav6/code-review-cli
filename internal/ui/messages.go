package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/LucasNav6/code-review-cli/internal/claude"
	"github.com/LucasNav6/code-review-cli/internal/githubpr"
	"github.com/LucasNav6/code-review-cli/internal/review"
)

var errClaudeStreamClosed = fmt.Errorf("Claude cerró el stream inesperadamente")

type prInfoLoadedMsg struct {
	info githubpr.Info
}

type diffLoadedMsg struct {
	diff string
}

type claudeStartedMsg struct {
	stage   int
	channel <-chan claude.Event
}

type claudeChunkMsg struct {
	stage int
	text  string
}

type claudeStatusMsg struct {
	stage int
	text  string
}

type claudeFinishedMsg struct {
	stage  int
	result string
}

type stageFailedMsg struct {
	stage int
	err   error
}

// commandFinishedMsg es el equivalente a claudeFinishedMsg para etapas
// review.KindCommand: ya trae el Result armado, no pasa por el parser de
// respuestas de Claude.
type commandFinishedMsg struct {
	stage     int
	result    *review.Result
	rawOutput string
}

type appErrorMsg struct {
	err error
}

// waitForClaudeEvent adapta el canal de eventos de internal/claude al loop
// de mensajes de bubbletea, leyendo un evento por vez.
func waitForClaudeEvent(stage int, channel <-chan claude.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-channel

		if !ok {
			return stageFailedMsg{
				stage: stage,
				err:   errClaudeStreamClosed,
			}
		}

		switch event.Type {
		case claude.EventChunk:
			return claudeChunkMsg{stage: stage, text: event.Text}

		case claude.EventStatus:
			return claudeStatusMsg{stage: stage, text: event.Text}

		case claude.EventFinished:
			return claudeFinishedMsg{stage: stage, result: event.Result}

		case claude.EventFailed:
			return stageFailedMsg{stage: stage, err: event.Err}

		default:
			return claudeStatusMsg{stage: stage, text: ""}
		}
	}
}
