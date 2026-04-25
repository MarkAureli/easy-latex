package pedantic

import (
	"fmt"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func init() {
	Register(Check{
		Name:   "single-spaces",
		Phase:  PhaseSource,
		Source: checkSingleSpaces,
		Fix:    fixSingleSpaces,
	})
}

// checkSingleSpaces flags runs of 2+ consecutive spaces past the leading
// whitespace of each line. Lines are comment-stripped by the runner.
func checkSingleSpaces(path string, lines []string) []Diagnostic {
	var diags []Diagnostic
	for i, line := range lines {
		if col := findMultiSpace(line); col >= 0 {
			diags = append(diags, Diagnostic{
				File:    path,
				Line:    i + 1,
				Message: fmt.Sprintf("multiple consecutive spaces at column %d", col+1),
			})
		}
	}
	return diags
}

// fixSingleSpaces collapses 2+ spaces to one in the non-comment body of each
// line. Leading whitespace and comment tail are preserved verbatim.
func fixSingleSpaces(path string, lines []string) ([]string, bool) {
	changed := false
	for i, line := range lines {
		body := texscan.StripComment(line)
		comment := line[len(body):]
		newBody := collapseSpaces(body)
		if newBody != body {
			lines[i] = newBody + comment
			changed = true
		}
	}
	return lines, changed
}

// findMultiSpace returns the 0-based index of the first run of 2+ spaces past
// any leading whitespace, or -1 if none.
func findMultiSpace(line string) int {
	start := 0
	for start < len(line) && (line[start] == ' ' || line[start] == '\t') {
		start++
	}
	for i := start; i+1 < len(line); i++ {
		if line[i] == ' ' && line[i+1] == ' ' {
			return i
		}
	}
	return -1
}

// collapseSpaces reduces runs of 2+ spaces to a single space in s, preserving
// any leading run of spaces and tabs.
func collapseSpaces(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	if start == len(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:start])
	prevSpace := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if c == ' ' {
			if prevSpace {
				continue
			}
			prevSpace = true
		} else {
			prevSpace = false
		}
		b.WriteByte(c)
	}
	return b.String()
}
