package pedantic

import (
	"fmt"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func init() {
	Register(Check{
		Name:   "block-on-newline",
		Phase:  PhaseSource,
		Source: checkBlockOnNewline,
		Fix:    fixBlockOnNewline,
	})
}

// blockKind classifies how a block token relates to its source line.
//   - blockLeading: the token must be the first non-whitespace on its line
//     (e.g. `\section`, `\begin{…}`, `\item`). Content/arguments may follow on
//     the same line.
//   - blockTrailing: the token must be the last non-whitespace on its line
//     (e.g. `\\` line break, `\newline`). Content before it may share the line.
type blockKind uint8

const (
	blockLeading blockKind = iota
	blockTrailing
)

// blockTokens maps tex commands subject to the block-on-newline rule to their
// kind. Token text includes the leading backslash and trailing star variant
// where applicable. Environment delimiters \begin{…} / \end{…} are leading
// and handled inline (see findBlockViolations).
var blockTokens = map[string]blockKind{
	// Display-math delimiters
	"\\[": blockLeading,
	"\\]": blockLeading,
	// Line breaks (must end the line, not start it)
	"\\\\":      blockTrailing,
	"\\newline": blockTrailing,
	// List item
	"\\item": blockLeading,
	// Sectioning
	"\\part": blockLeading, "\\part*": blockLeading,
	"\\chapter": blockLeading, "\\chapter*": blockLeading,
	"\\section": blockLeading, "\\section*": blockLeading,
	"\\subsection": blockLeading, "\\subsection*": blockLeading,
	"\\subsubsection": blockLeading, "\\subsubsection*": blockLeading,
	"\\paragraph": blockLeading, "\\paragraph*": blockLeading,
	"\\subparagraph": blockLeading, "\\subparagraph*": blockLeading,
	// Page / vertical-space breaks
	"\\newpage":   blockLeading,
	"\\clearpage": blockLeading,
	"\\pagebreak": blockLeading,
	"\\linebreak": blockLeading,
	"\\bigskip":   blockLeading,
	"\\medskip":   blockLeading,
	"\\smallskip": blockLeading,
	"\\vspace":    blockLeading, "\\vspace*": blockLeading,
	// Float meta
	"\\caption": blockLeading, "\\caption*": blockLeading,
	// File inclusion
	"\\input":   blockLeading,
	"\\include": blockLeading,
	"\\subfile": blockLeading,
	"\\import":  blockLeading,
	// Front / back matter
	"\\maketitle":         blockLeading,
	"\\tableofcontents":   blockLeading,
	"\\listoffigures":     blockLeading,
	"\\listoftables":      blockLeading,
	"\\printbibliography": blockLeading,
	"\\bibliography":      blockLeading,
	"\\appendix":          blockLeading,
	"\\frontmatter":       blockLeading,
	"\\mainmatter":        blockLeading,
	"\\backmatter":        blockLeading,
	// Preamble
	"\\documentclass":       blockLeading,
	"\\usepackage":          blockLeading,
	"\\title":               blockLeading,
	"\\author":              blockLeading,
	"\\date":                blockLeading,
	"\\newcommand":          blockLeading,
	"\\renewcommand":        blockLeading,
	"\\newenvironment":      blockLeading,
	"\\DeclareMathOperator": blockLeading,
	"\\theoremstyle":        blockLeading,
	"\\newtheorem":          blockLeading,
	// Tabular rules
	"\\hline":      blockLeading,
	"\\midrule":    blockLeading,
	"\\toprule":    blockLeading,
	"\\bottomrule": blockLeading,
	"\\cmidrule":   blockLeading,
}

func checkBlockOnNewline(path string, lines []string) []Diagnostic {
	mask := regionMask(lines)
	var diags []Diagnostic
	for li, line := range lines {
		for _, v := range findBlockViolations(line, mask[li]) {
			diags = append(diags, Diagnostic{
				File:    path,
				Line:    li + 1,
				Message: v.message(),
			})
		}
	}
	return diags
}

func fixBlockOnNewline(path string, lines []string) ([]string, bool) {
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = texscan.StripComment(l)
	}
	mask := regionMask(stripped)
	out := make([]string, 0, len(lines))
	changed := false
	for li, line := range lines {
		body := stripped[li]
		comment := line[len(body):]
		violations := findBlockViolations(body, mask[li])
		if len(violations) == 0 {
			out = append(out, line)
			continue
		}
		splits := make([]int, len(violations))
		for k, v := range violations {
			splits[k] = v.split
		}
		out = append(out, splitLineAt(body, comment, body[:leadingWS(body)], splits)...)
		changed = true
	}
	return out, changed
}

type blockViolation struct {
	tok    string    // token text (with leading backslash)
	tokCol int       // 1-based column of the offending token
	split  int       // byte offset at which to insert a newline
	kind   blockKind // leading or trailing
}

func (v blockViolation) message() string {
	switch v.kind {
	case blockTrailing:
		return fmt.Sprintf("block-level token %s should end the line; content after it should start a new line (column %d)", v.tok, v.split+1)
	default:
		return fmt.Sprintf("block-level token %s should start a new line (column %d)", v.tok, v.tokCol)
	}
}

// findBlockViolations walks line and returns block-token placement issues.
// Math and verbatim regions are skipped (they have their own conventions —
// e.g. tabular/matrix `\\` row separators, `\begin{cases}` mid-equation).
func findBlockViolations(line string, mask []regionKind) []blockViolation {
	var out []blockViolation
	leadEnd := leadingWS(line)
	i := leadEnd
	for i < len(line) {
		if i < len(mask) && mask[i] != regText {
			i++
			continue
		}
		if line[i] != '\\' {
			i++
			continue
		}
		tok, n := nextTokenAt(line, i)
		if n == 0 {
			i++
			continue
		}
		kind, isBlock := blockTokens[tok]
		isEnv := tok == "\\begin" || tok == "\\end"
		if !isBlock && !isEnv {
			i += n
			continue
		}
		if isEnv {
			kind = blockLeading
		}
		// Advance past the token (and the env brace group, if applicable).
		end := i + n
		if isEnv && end < len(line) && line[end] == '{' {
			if br := strings.IndexByte(line[end:], '}'); br >= 0 {
				end += br + 1
			}
		}
		switch kind {
		case blockLeading:
			// Allow leading tokens preceded only by whitespace and `{` so that
			// macro-definition bodies like `{\end{subequations}}` (used in
			// \NewDocumentEnvironment) are not flagged.
			if !precededByOnlyOpenGrouping(line, i) {
				out = append(out, blockViolation{
					tok:    tok,
					tokCol: i + 1,
					split:  i,
					kind:   blockLeading,
				})
			}
		case blockTrailing:
			tail := end
			for tail < len(line) && (line[tail] == ' ' || line[tail] == '\t') {
				tail++
			}
			if tail < len(line) {
				out = append(out, blockViolation{
					tok:    tok,
					tokCol: i + 1,
					split:  tail,
					kind:   blockTrailing,
				})
			}
		}
		i = end
	}
	return out
}

// nextTokenAt parses a tex command starting at line[i] (which must be a
// backslash). Letter commands consume `\name` plus an optional trailing `*`;
// non-letter commands consume two bytes (e.g. `\\`, `\[`, `\]`).
func nextTokenAt(line string, i int) (string, int) {
	if i >= len(line) || line[i] != '\\' {
		return "", 0
	}
	if i+1 >= len(line) {
		return "", 0
	}
	c := line[i+1]
	if isLetter(c) {
		j := i + 1
		for j < len(line) && isLetter(line[j]) {
			j++
		}
		if j < len(line) && line[j] == '*' {
			j++
		}
		return line[i:j], j - i
	}
	return line[i : i+2], 2
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// precededByOnlyOpenGrouping reports whether all bytes in line[:i] are spaces,
// tabs, or `{`. This treats macro-body group openings as transparent indent.
func precededByOnlyOpenGrouping(line string, i int) bool {
	for j := range i {
		c := line[j]
		if c != ' ' && c != '\t' && c != '{' {
			return false
		}
	}
	return true
}
