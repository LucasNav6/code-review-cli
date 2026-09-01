package logging

import (
	"bytes"
	"errors"
	"regexp"
	"testing"
	"time"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestLogErrorFormatsAndReturnsError(t *testing.T) {
	var out bytes.Buffer
	err := errors.New("pull request URL is required")

	gotErr := LogError(&out, ErrorTypeInput, 1, err)

	if gotErr == nil {
		t.Fatal("expected returned error")
	}
	if !errors.Is(gotErr, err) {
		t.Fatal("returned error should preserve sentinel matching")
	}

	got := ansiRE.ReplaceAllString(out.String(), "")
	matched, err := regexp.MatchString(`^ERRO \([0-9]{2}:[0-9]{2}\) Exit status \(1\)\n╰─▶ pull request URL is required\n$`, got)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestLogErrorDoesNotDuplicateAlreadyLoggedError(t *testing.T) {
	var first bytes.Buffer
	err := LogError(&first, ErrorTypeInput, 1, errors.New("failed"))

	var second bytes.Buffer
	LogError(&second, ErrorTypeUnknown, 1, err)

	if second.Len() != 0 {
		t.Fatalf("expected no duplicate log, got %q", second.String())
	}
}

func TestLogInfoFormat(t *testing.T) {
	var out bytes.Buffer
	logger := newLogger(&out, func() time.Time {
		return time.Date(2026, 9, 1, 12, 34, 0, 0, time.UTC)
	})

	logger.info("diff stored", "url", "https://github.com/org/repo/pull/123", "path", "/tmp/review.diff")

	got := ansiRE.ReplaceAllString(out.String(), "")
	want := "INFO (12:34) diff stored\n | url=https://github.com/org/repo/pull/123\n | path=/tmp/review.diff\n"
	if got != want {
		t.Fatalf("unexpected output:\nwant %q\ngot  %q", want, got)
	}
}
