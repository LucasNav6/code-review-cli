// Package claude ejecuta el CLI de Claude Code en modo streaming y traduce
// su salida `stream-json` a una secuencia de eventos simple, sin ninguna
// dependencia de la capa de interfaz.
package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// EventType distingue los distintos eventos que puede emitir una ejecución.
type EventType int

const (
	// EventChunk es un fragmento de texto generado por el modelo.
	EventChunk EventType = iota
	// EventStatus es una novedad operativa (por ejemplo, un reintento).
	EventStatus
	// EventFinished indica que la ejecución terminó con éxito.
	EventFinished
	// EventFailed indica que la ejecución falló.
	EventFailed
)

// Event es un mensaje emitido durante el streaming de una ejecución.
type Event struct {
	Type EventType

	// Text es el contenido para EventChunk y EventStatus.
	Text string

	// Result es la respuesta final para EventFinished.
	Result string

	// Err es el error para EventFailed.
	Err error
}

// ErrNotInstalled se devuelve cuando el binario `claude` no está disponible.
var ErrNotInstalled = fmt.Errorf(
	"no encontré el comando \"claude\" (Claude Code CLI). Instalalo desde https://docs.claude.com/claude-code",
)

type streamEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Result  string `json:"result"`

	Attempt      int `json:"attempt"`
	MaxRetries   int `json:"max_retries"`
	RetryDelayMS int `json:"retry_delay_ms"`

	Event struct {
		Type string `json:"type"`

		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
}

// Stream ejecuta `claude -p` con el prompt dado y devuelve un canal de
// eventos. El canal se cierra automáticamente cuando la ejecución termina,
// ya sea con éxito o con error.
func Stream(prompt string) <-chan Event {
	channel := make(chan Event)

	go run(prompt, channel)

	return channel
}

func run(prompt string, channel chan<- Event) {
	defer close(channel)

	if _, err := exec.LookPath("claude"); err != nil {
		channel <- Event{Type: EventFailed, Err: ErrNotInstalled}
		return
	}

	cmd := exec.Command(
		"claude",
		"-p",
		"--output-format",
		"stream-json",
		"--verbose",
		"--include-partial-messages",
	)

	cmd.Stdin = strings.NewReader(prompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		channel <- Event{Type: EventFailed, Err: err}
		return
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		channel <- Event{
			Type: EventFailed,
			Err:  fmt.Errorf("no pude iniciar Claude: %w", err),
		}

		return
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)

	var generated strings.Builder
	var finalResult string

	for scanner.Scan() {
		var event streamEvent

		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}

		switch {
		case event.Type == "stream_event" && event.Event.Delta.Type == "text_delta":
			text := event.Event.Delta.Text

			if text == "" {
				continue
			}

			generated.WriteString(text)

			channel <- Event{Type: EventChunk, Text: text}

		case event.Type == "system" && event.Subtype == "api_retry":
			delaySeconds := float64(event.RetryDelayMS) / 1000

			channel <- Event{
				Type: EventStatus,
				Text: fmt.Sprintf(
					"Claude tuvo un problema temporal. Reintentando %d/%d en %.1fs...",
					event.Attempt,
					event.MaxRetries,
					delaySeconds,
				),
			}

		case event.Type == "result":
			if event.Result != "" {
				finalResult = event.Result
			}
		}
	}

	if err := scanner.Err(); err != nil {
		_ = cmd.Process.Kill()

		channel <- Event{
			Type: EventFailed,
			Err:  fmt.Errorf("se interrumpió la salida de Claude: %w", err),
		}

		return
	}

	if err := cmd.Wait(); err != nil {
		message := strings.TrimSpace(stderr.String())

		if message == "" {
			message = err.Error()
		}

		channel <- Event{
			Type: EventFailed,
			Err:  fmt.Errorf("Claude no pudo completar la revisión: %s", message),
		}

		return
	}

	if strings.TrimSpace(finalResult) == "" {
		finalResult = generated.String()
	}

	channel <- Event{
		Type:   EventFinished,
		Result: strings.TrimSpace(finalResult),
	}
}
