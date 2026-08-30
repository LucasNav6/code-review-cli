package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/LucasNav6/code-review-cli/internal/review"
)

type diffLine struct {
	Kind      byte
	OldLine   int
	NewLine   int
	Content   string
	Highlight bool
}

type diffBlock struct {
	File      string
	Lines     []diffLine
	Additions int
	Deletions int
}

func renderInlineReview(diff string, findings []review.Finding, width int) string {
	if len(findings) == 0 {
		return renderFilesChanged(diff, nil, width)
	}

	groups := groupFindingsByFile(findings)
	blocks := parseDiffBlocks(diff, groups, false)

	var b strings.Builder

	b.WriteString(mutedStyle.Render(fmt.Sprintf("%d inline comments", len(findings))))

	renderedFiles := map[string]bool{}

	for _, block := range blocks {
		fileFindings := groups[block.File]
		if len(fileFindings) == 0 {
			continue
		}

		renderedFiles[block.File] = true

		b.WriteString("\n\n")
		b.WriteString(renderDiffFile(block, fileFindings, width, true))
	}

	for file, fileFindings := range groups {
		if renderedFiles[file] {
			continue
		}

		b.WriteString("\n\n")
		b.WriteString(renderUnmatchedFindings(file, fileFindings, width))
	}

	return b.String()
}

func renderFilesChanged(diff string, findings []review.Finding, width int) string {
	groups := groupFindingsByFile(findings)
	blocks := parseDiffBlocks(diff, groups, true)

	if len(blocks) == 0 {
		return mutedStyle.Render("No diff content available.")
	}

	var b strings.Builder

	b.WriteString(mutedStyle.Render(fmt.Sprintf("%d files changed", len(blocks))))

	for _, block := range blocks {
		b.WriteString("\n\n")
		b.WriteString(renderDiffFile(block, groups[block.File], width, false))
	}

	return b.String()
}

func groupFindingsByFile(findings []review.Finding) map[string][]review.Finding {
	groups := map[string][]review.Finding{}

	for _, finding := range findings {
		file := normalizeDiffPath(finding.File)
		if file == "" {
			file = "General review"
		}

		groups[file] = append(groups[file], finding)
	}

	return groups
}

func parseDiffBlocks(diff string, groups map[string][]review.Finding, includeAll bool) []diffBlock {
	var blocks []diffBlock
	var currentFile string
	var previousFile string
	var currentLines []diffLine
	var oldLine, newLine int
	inCurrentFile := false

	flush := func() {
		if currentFile != "" && len(currentLines) > 0 {
			additions, deletions := diffStats(currentLines)
			blocks = append(blocks, diffBlock{
				File:      currentFile,
				Lines:     currentLines,
				Additions: additions,
				Deletions: deletions,
			})
		}

		currentLines = nil
	}

	for _, rawLine := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(rawLine, "diff --git "):
			flush()
			currentFile = ""
			previousFile = ""
			inCurrentFile = false

		case strings.HasPrefix(rawLine, "--- "):
			previousFile = normalizeDiffPath(strings.TrimPrefix(rawLine, "--- "))

		case strings.HasPrefix(rawLine, "+++ "):
			currentFile = normalizeDiffPath(strings.TrimPrefix(rawLine, "+++ "))
			if currentFile == "" {
				currentFile = previousFile
			}

			_, hasFindings := groups[currentFile]
			inCurrentFile = includeAll || hasFindings

		case strings.HasPrefix(rawLine, "@@"):
			if !inCurrentFile {
				continue
			}

			startOld, startNew, ok := parseHunkHeader(rawLine)
			if !ok {
				continue
			}

			oldLine = startOld
			newLine = startNew
			currentLines = append(currentLines, diffLine{Kind: '@', Content: rawLine})

		case inCurrentFile && strings.HasPrefix(rawLine, "+"):
			line := diffLine{Kind: '+', NewLine: newLine, Content: strings.TrimPrefix(rawLine, "+")}
			currentLines = append(currentLines, line)
			newLine++

		case inCurrentFile && strings.HasPrefix(rawLine, "-"):
			line := diffLine{Kind: '-', OldLine: oldLine, Content: strings.TrimPrefix(rawLine, "-")}
			currentLines = append(currentLines, line)
			oldLine++

		case inCurrentFile && strings.HasPrefix(rawLine, " "):
			line := diffLine{Kind: ' ', OldLine: oldLine, NewLine: newLine, Content: strings.TrimPrefix(rawLine, " ")}
			currentLines = append(currentLines, line)
			oldLine++
			newLine++
		}
	}

	flush()
	markCommentLines(blocks, groups)

	return blocks
}

func parseHunkHeader(header string) (int, int, bool) {
	fields := strings.Fields(header)
	if len(fields) < 3 {
		return 0, 0, false
	}

	oldStart, ok := parseRangeStart(fields[1], "-")
	if !ok {
		return 0, 0, false
	}

	newStart, ok := parseRangeStart(fields[2], "+")
	if !ok {
		return 0, 0, false
	}

	return oldStart, newStart, true
}

func parseRangeStart(value string, prefix string) (int, bool) {
	value = strings.TrimPrefix(value, prefix)
	value = strings.Split(value, ",")[0]

	line, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}

	return line, true
}

func markCommentLines(blocks []diffBlock, groups map[string][]review.Finding) {
	for blockIndex := range blocks {
		lineNumbers := map[int]bool{}

		for _, finding := range groups[blocks[blockIndex].File] {
			if finding.Line > 0 {
				lineNumbers[finding.Line] = true
			}
		}

		for lineIndex := range blocks[blockIndex].Lines {
			if lineNumbers[blocks[blockIndex].Lines[lineIndex].NewLine] {
				blocks[blockIndex].Lines[lineIndex].Highlight = true
			}
		}
	}
}

func renderDiffFile(block diffBlock, findings []review.Finding, width int, compact bool) string {
	title := renderFileRule(block, width)
	lines := block.Lines
	if compact && len(findings) > 0 {
		lines = compactDiffLines(block.Lines, findings)
	}

	diffBody := renderDiffLines(block.File, lines, findings, width)

	return title + "\n" + normalPanel.Width(width).Render(diffBody)
}

func renderFileRule(block diffBlock, width int) string {
	label := strings.Join([]string{
		" " + block.File,
		additionStatStyle.Render(fmt.Sprintf("+%d", block.Additions)),
		deletionStatStyle.Render(fmt.Sprintf("-%d", block.Deletions)),
		"",
	}, " ")
	labelWidth := lipgloss.Width(label)
	if labelWidth >= width {
		return fileHeaderStyle.Render(truncateRunes(block.File, width))
	}

	return fileHeaderStyle.Render(label + strings.Repeat("─", width-labelWidth))
}

func diffStats(lines []diffLine) (int, int) {
	var additions, deletions int

	for _, line := range lines {
		switch line.Kind {
		case '+':
			additions++
		case '-':
			deletions++
		}
	}

	return additions, deletions
}

func renderDiffLines(file string, lines []diffLine, findings []review.Finding, width int) string {
	innerWidth := max(24, width-4)
	maxLineWidth := max(24, innerWidth-10)
	findingsByLine := groupFindingsByLine(findings)
	var b strings.Builder

	for _, line := range lines {
		if line.Kind == '@' {
			b.WriteString(hunkStyle.Width(innerWidth).Render("    " + truncateRunes(line.Content, maxLineWidth)))
			b.WriteString("\n")
			continue
		}

		oldNo := lineNumberCell(line.OldLine)
		newNo := lineNumberCell(line.NewLine)
		prefix := diffPrefix(line.Kind)
		content := highlightCode(file, truncateRunes(line.Content, maxLineWidth))
		row := fmt.Sprintf("%s %s  %s %s", oldNo, newNo, prefix, content)

		switch line.Kind {
		case '+':
			row = addedLineStyle.Width(innerWidth).Render(row)
		case '-':
			row = deletedLineStyle.Width(innerWidth).Render(row)
		default:
			row = contextLineStyle.Width(innerWidth).Render(row)
		}

		if line.Highlight {
			row = commentTargetStyle.Width(innerWidth).Render(row)
		}

		b.WriteString(row)
		b.WriteString("\n")

		for _, finding := range findingsByLine[line.NewLine] {
			b.WriteString(renderInlineComment(finding, innerWidth))
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func groupFindingsByLine(findings []review.Finding) map[int][]review.Finding {
	groups := map[int][]review.Finding{}

	for _, finding := range findings {
		if finding.Line > 0 {
			groups[finding.Line] = append(groups[finding.Line], finding)
		}
	}

	return groups
}

func diffPrefix(kind byte) string {
	switch kind {
	case '+':
		return additionStatStyle.Render("+")
	case '-':
		return deletionStatStyle.Render("-")
	default:
		return " "
	}
}

func renderFileComments(findings []review.Finding, width int) string {
	var b strings.Builder

	for _, finding := range findings {
		b.WriteString("\n")
		b.WriteString(renderReviewComment(finding, width-4))
	}

	return b.String()
}

func renderInlineComment(finding review.Finding, width int) string {
	const gutterWidth = 12

	commentWidth := min(max(52, width-gutterWidth-4), 110)
	comment := renderReviewComment(finding, commentWidth)
	prefix := commentLineStyle.Render(strings.Repeat(" ", gutterWidth))

	var b strings.Builder
	for _, line := range strings.Split(comment, "\n") {
		b.WriteString(prefix)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func renderReviewComment(finding review.Finding, width int) string {
	bodyLines := []string{}

	if strings.TrimSpace(finding.Comment) != "" {
		bodyLines = append(bodyLines, commentBodyStyle.Render(wrapText(finding.Comment, width-3)))
	}

	for _, detail := range finding.Details {
		if strings.TrimSpace(detail.Value) == "" {
			continue
		}

		bodyLines = append(bodyLines, commentMutedStyle.Render(detail.Label+":"))
		bodyLines = append(bodyLines, commentBodyStyle.Render(wrapText(detail.Value, width-3)))
	}

	if finding.Suggestion != "" {
		bodyLines = append(
			bodyLines,
			commentMutedStyle.Render("suggestion:")+" "+commentBodyStyle.Render(wrapText(finding.Suggestion, width-15)),
		)
	}

	if len(bodyLines) == 0 {
		bodyLines = append(bodyLines, commentMutedStyle.Render("Sin comentario estructurado. Usá Tab para ver la salida completa."))
	}

	author := commentHeaderStyle.Width(width - 4).Render("@code-review (agent)")
	category := renderCommentCategoryRule(finding.Category, width-4)
	body := indentCommentBody(bodyLines)

	return commentPanel.Width(width).Render(author + "\n" + category + "\n" + body)
}

func renderCommentCategoryRule(category string, width int) string {
	label := "[" + strings.ToUpper(strings.TrimSpace(category)) + "]"
	if label == "[]" {
		label = "[REVIEW]"
	}

	labelWidth := lipgloss.Width(label)
	if labelWidth >= width {
		return commentCategoryStyle.Render(truncateRunes(label, width))
	}

	return commentCategoryStyle.Render(label + strings.Repeat("─", width-labelWidth))
}

func indentCommentBody(lines []string) string {
	var b strings.Builder

	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}

		b.WriteString("  ")
		b.WriteString(line)
	}

	return b.String()
}

func renderUnmatchedFindings(file string, findings []review.Finding, width int) string {
	var b strings.Builder

	b.WriteString(fileHeaderStyle.Width(width).Render("▾  " + file))

	for _, finding := range findings {
		b.WriteString("\n")
		b.WriteString(renderReviewComment(finding, width-4))
	}

	return normalPanel.Width(width).Render(b.String())
}

func normalizeDiffPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")

	if path == "/dev/null" {
		return ""
	}

	return path
}

func lineNumberCell(line int) string {
	if line <= 0 {
		return dimStyle.Render("   ·")
	}

	return mutedStyle.Render(fmt.Sprintf("%4d", line))
}

func compactDiffLines(lines []diffLine, findings []review.Finding) []diffLine {
	if len(lines) == 0 {
		return nil
	}

	targets := map[int]bool{}
	for _, finding := range findings {
		if finding.Line > 0 {
			targets[finding.Line] = true
		}
	}

	if len(targets) == 0 {
		return lines
	}

	const context = 4

	keep := map[int]bool{}
	lastHunk := 0

	for i, line := range lines {
		if line.Kind == '@' {
			lastHunk = i
			continue
		}

		if !targets[line.NewLine] {
			continue
		}

		keep[lastHunk] = true

		start := max(lastHunk+1, i-context)
		end := min(len(lines)-1, i+context)

		for j := start; j <= end; j++ {
			keep[j] = true
		}
	}

	compacted := make([]diffLine, 0, len(keep))
	previousKept := -1

	for i, line := range lines {
		if !keep[i] {
			continue
		}

		if previousKept != -1 && i > previousKept+1 {
			compacted = append(compacted, diffLine{Kind: '@', Content: "..."})
		}

		compacted = append(compacted, line)
		previousKept = i
	}

	if len(compacted) == 0 {
		return lines
	}

	return compacted
}

func truncateRunes(value string, maxWidth int) string {
	if maxWidth <= 1 {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= maxWidth {
		return value
	}

	return string(runes[:maxWidth-1]) + "…"
}
