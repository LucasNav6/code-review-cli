package logging

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// Log levels exposed as public constants so callers (and tests) can refer
// to them without depending on log/slog directly.
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

const logTimeFormat = "15:04"

// logger is a thin styled-log facade. Internally it uses log/slog for
// level filtering and lipgloss styles for output colouring, but the
// on-disk format is owned by this package so it is stable for tests
// and downstream consumers.
type logger struct {
	out   io.Writer
	now   func() time.Time
	level slog.Level
	style styles
}

type styles struct {
	debug lipgloss.Style
	info  lipgloss.Style
	warn  lipgloss.Style
	error lipgloss.Style
	key   lipgloss.Style
}

type loggedError struct {
	errorType ErrorType
	err       error
	logged    bool
}

func (e loggedError) Error() string {
	return e.err.Error()
}

func (e loggedError) Unwrap() error {
	return e.err
}

// LogError writes a styled error line to w and returns an error that
// wraps err. If err is already a loggedError that was previously
// emitted, no duplicate log is produced and the same error is returned
// unchanged. This is how the package prevents double-logging when a
// lower layer already surfaced the failure.
func LogError(w io.Writer, errorType ErrorType, code int, err error) error {
	if err == nil {
		return nil
	}

	var logged loggedError
	if errors.As(err, &logged) {
		if !logged.logged {
			newLogger(w, time.Now).logError(logged.errorType, code, logged.err)
			logged.logged = true
		}
		return logged
	}

	newLogger(w, time.Now).logError(errorType, code, err)
	return loggedError{
		errorType: errorType,
		err:       err,
		logged:    true,
	}
}

func LogDebug(w io.Writer, message string, keyvals ...any) {
	newLogger(w, time.Now).debug(message, keyvals...)
}

func LogInfo(w io.Writer, message string, keyvals ...any) {
	newLogger(w, time.Now).info(message, keyvals...)
}

func LogWarn(w io.Writer, message string, keyvals ...any) {
	newLogger(w, time.Now).warn(message, keyvals...)
}

// newLogger builds a logger that writes to w using the current time
// from now. If w is nil, os.Stderr is used. The minimum accepted
// level is Debug so all messages flow through the format layer; the
// caller decides whether to actually emit by configuring a higher
// level via SetLevel.
func newLogger(w io.Writer, now func() time.Time) *logger {
	if w == nil {
		w = os.Stderr
	}

	return &logger{
		out:   w,
		now:   now,
		level: LevelDebug,
		style: styles{
			debug: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#58A6FF")),
			info:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#3FB950")),
			warn:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#D29922")),
			error: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F85149")),
			key:   lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E")),
		},
	}
}

// SetLevel overrides the minimum level that the logger will emit.
func (l *logger) SetLevel(level slog.Level) {
	l.level = level
}

func (l *logger) logError(errorType ErrorType, code int, err error) {
	if err == nil || !l.enabled(LevelError) {
		return
	}
	_ = errorType

	lineStyle := l.style.error
	fmt.Fprintln(l.out, lineStyle.Render(fmt.Sprintf("ERRO (%s) Exit status (%d)", l.timestamp(), code)))
	fmt.Fprintln(l.out, lineStyle.Render("╰─▶ "+err.Error()))
}

func (l *logger) debug(message string, keyvals ...any) {
	l.log(LevelDebug, "DEBU", message, keyvals...)
}

func (l *logger) info(message string, keyvals ...any) {
	l.log(LevelInfo, "INFO", message, keyvals...)
}

func (l *logger) warn(message string, keyvals ...any) {
	l.log(LevelWarn, "WARN", message, keyvals...)
}

func (l *logger) log(level slog.Level, label string, message string, keyvals ...any) {
	if !l.enabled(level) {
		return
	}

	var style lipgloss.Style
	switch level {
	case LevelDebug:
		style = l.style.debug
	case LevelInfo:
		style = l.style.info
	case LevelWarn:
		style = l.style.warn
	case LevelError:
		style = l.style.error
	}

	fmt.Fprintln(l.out, style.Render(fmt.Sprintf("%s (%s)", label, l.timestamp()))+" "+message)

	for i := 0; i < len(keyvals); i += 2 {
		key := fmt.Sprint(keyvals[i])
		value := "<missing>"
		if i+1 < len(keyvals) {
			value = fmt.Sprint(keyvals[i+1])
		}

		if strings.TrimSpace(key) == "" {
			continue
		}

		fmt.Fprintln(l.out, l.style.key.Render(fmt.Sprintf(" | %s=%s", key, value)))
	}
}

func (l *logger) enabled(level slog.Level) bool {
	return l.level <= level
}

func (l *logger) timestamp() string {
	return l.now().Format(logTimeFormat)
}
