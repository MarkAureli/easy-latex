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

// alignmentTerminator reports whether c is a character that is conventionally
// preceded by alignment spacing in LaTeX source: `=` for key-value macro
// arguments (e.g. `\hypersetup{colorlinks  = true}`) and `&` for tabular and
// align-style column separators.
func alignmentTerminator(c byte) bool {
	return c == '=' || c == '&'
}

// findMultiSpace returns the 0-based index of the first run of 2+ spaces past
// any leading whitespace, or -1 if none. Runs are ignored when they are part
// of the line's trailing whitespace (alignment spaces before a stripped `%`
// comment) or when they are immediately followed by an alignment terminator
// (`=`, `&`).
func findMultiSpace(line string) int {
	end := trailingWSStart(line)
	i := leadingWS(line)
	for i+1 < end {
		if line[i] != ' ' || line[i+1] != ' ' {
			i++
			continue
		}
		j := i + 2
		for j < end && line[j] == ' ' {
			j++
		}
		if j >= end || !alignmentTerminator(line[j]) {
			return i
		}
		i = j
	}
	return -1
}

// collapseSpaces reduces runs of 2+ spaces to a single space in s, preserving
// any leading run of spaces and tabs, any trailing whitespace, and any run
// whose terminator is an alignment character (`=`, `&`).
func collapseSpaces(s string) string {
	start := leadingWS(s)
	end := trailingWSStart(s)
	if start >= end {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:start])
	for i := start; i < end; {
		if s[i] != ' ' {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i + 1
		for j < end && s[j] == ' ' {
			j++
		}
		runLen := j - i
		if runLen >= 2 && j < end && alignmentTerminator(s[j]) {
			b.WriteString(s[i:j])
		} else {
			b.WriteByte(' ')
		}
		i = j
	}
	b.WriteString(s[end:])
	return b.String()
}

// trailingWSStart returns the byte offset of the first character of the
// line's trailing run of spaces and tabs, or len(s) if the line has no
// trailing whitespace.
func trailingWSStart(s string) int {
	i := len(s)
	for i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
		i--
	}
	return i
}
