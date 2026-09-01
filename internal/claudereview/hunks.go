package claudereview

import (
	"regexp"
	"strconv"
	"strings"
)

// Hunk is a single `@@ -old,len +new,len @@` block from a unified
// diff. OldStart / NewStart are 1-indexed; OldLines / NewLines are
// the lengths as reported by the hunk header (the diff may include
// fewer actual content lines than the header claims when the diff
// ends with EOF).
//
// FilePath is the path that appeared in the most recent `+++ b/...`
// header before this hunk. Empty until the first file is processed.
type Hunk struct {
	FilePath string
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Header   string   // everything after the second `@@`, e.g. function name
	Lines    []string // raw diff lines including the leading +/-/' '
}

// hunkHeaderRE captures the four integers out of an `@@ -a,b +c,d @@`
// header. The trailing `@@` is matched but not captured.
var hunkHeaderRE = regexp.MustCompile(`^@@\s+-(\d+)(?:,(\d+))?\s+\+(\d+)(?:,(\d+))?\s+@@(.*)$`)

// newFileRE captures the destination path from a `+++ b/path` line.
// The "b/" prefix is what git-style diffs use.
var newFileRE = regexp.MustCompile(`^\+\+\+\s+(?:b/)?(.+?)\s*$`)

// ParseHunks walks a unified diff and returns one Hunk per `@@` block.
// Lines that do not belong to any hunk (file headers, "diff --git",
// "index", "---", "+++") update the current file context but are not
// themselves part of any hunk body.
func ParseHunks(diff string) []Hunk {
	var hunks []Hunk
	var current *Hunk
	var currentPath string

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			if current != nil {
				hunks = append(hunks, *current)
			}
			matches := hunkHeaderRE.FindStringSubmatch(line)
			if matches == nil {
				current = nil
				continue
			}

			oldStart, _ := strconv.Atoi(matches[1])
			oldLen, _ := strconv.Atoi(matches[2])
			newStart, _ := strconv.Atoi(matches[3])
			newLen, _ := strconv.Atoi(matches[4])

			current = &Hunk{
				FilePath: currentPath,
				OldStart: oldStart,
				OldLines: oldLen,
				NewStart: newStart,
				NewLines: newLen,
				Header:   strings.TrimSpace(matches[5]),
				Lines:    nil,
			}

		case strings.HasPrefix(line, "+++ "):
			// Closing any previous hunk is handled by the @@ branch
			// above; here we only update the file context.
			matches := newFileRE.FindStringSubmatch(line)
			if matches != nil {
				currentPath = matches[1]
				if current != nil {
					current.FilePath = currentPath
				}
			}

		case strings.HasPrefix(line, "diff --git "),
			strings.HasPrefix(line, "index "),
			strings.HasPrefix(line, "--- "):
			// File metadata lines do not contribute to a hunk body.
			continue
		}

		if current == nil {
			continue
		}

		// Skip the metadata lines we already classified above; the
		// rest of the iteration appends raw diff lines.
		if strings.HasPrefix(line, "diff --git ") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "@@") {
			continue
		}

		current.Lines = append(current.Lines, line)
	}

	if current != nil {
		hunks = append(hunks, *current)
	}

	return hunks
}

// HunkForLine returns the first hunk in hunks whose NewStart..NewStart
// +NewLines range covers newLine AND whose FilePath matches the
// supplied path. Both conditions must hold so we never pull a hunk
// from a different file just because the line numbers line up.
func HunkForLine(hunks []Hunk, path string, newLine int) (Hunk, bool) {
	for _, h := range hunks {
		if h.FilePath != path {
			continue
		}
		if newLine < h.NewStart {
			continue
		}
		if newLine <= h.NewStart+h.NewLines {
			return h, true
		}
	}
	return Hunk{}, false
}
