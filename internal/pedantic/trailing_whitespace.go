package pedantic

import "fmt"

func init() {
	Register(Check{
		Name:    "no-trailing-whitespace",
		Phase:   PhaseSource,
		Source:  checkTrailingWhitespace,
		Fix:     fixTrailingWhitespace,
		WantRaw: true,
	})
}

// checkTrailingWhitespace flags spaces and tabs at the end of any raw source
// line. For lines with comments, this targets whitespace after the comment
// text; for code-only lines, whitespace at the end of the code. Pre-comment
// alignment whitespace (spaces between code and the `%` marker) is the domain
// of single-spaces and is not considered trailing.
func checkTrailingWhitespace(path string, lines []string) []Diagnostic {
	var diags []Diagnostic
	for i, line := range lines {
		end := trimRightWS(line)
		if end == len(line) {
			continue
		}
		diags = append(diags, Diagnostic{
			File:    path,
			Line:    i + 1,
			Message: fmt.Sprintf("trailing whitespace at column %d", end+1),
		})
	}
	return diags
}

// fixTrailingWhitespace strips trailing spaces and tabs from each raw line.
func fixTrailingWhitespace(path string, lines []string) ([]string, bool) {
	changed := false
	for i, line := range lines {
		end := trimRightWS(line)
		if end == len(line) {
			continue
		}
		lines[i] = line[:end]
		changed = true
	}
	return lines, changed
}

// trimRightWS returns the byte offset just past the last non-whitespace byte
// in s, considering only ' ' and '\t' as whitespace. Returns len(s) when s
// has no trailing whitespace.
func trimRightWS(s string) int {
	end := len(s)
	for end > 0 && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return end
}
