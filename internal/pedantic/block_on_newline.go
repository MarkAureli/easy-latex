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

// blockTokens lists tex commands that should always start a new source line.
// Token text includes the leading backslash and trailing star variant where
// applicable. Environment delimiters \begin{...} / \end{...} are handled
// inline via the explicit `\begin`/`\end` check in findBlockViolations.
var blockTokens = map[string]bool{
	// Display-math delimiters
	"\\[": true,
	"\\]": true,
	// Line breaks
	"\\\\":      true,
	"\\newline": true,
	// List item
	"\\item": true,
	// Sectioning
	"\\part": true, "\\part*": true,
	"\\chapter": true, "\\chapter*": true,
	"\\section": true, "\\section*": true,
	"\\subsection": true, "\\subsection*": true,
	"\\subsubsection": true, "\\subsubsection*": true,
	"\\paragraph": true, "\\paragraph*": true,
	"\\subparagraph": true, "\\subparagraph*": true,
	// Page / vertical-space breaks
	"\\newpage":   true,
	"\\clearpage": true,
	"\\pagebreak": true,
	"\\linebreak": true,
	"\\bigskip":   true,
	"\\medskip":   true,
	"\\smallskip": true,
	"\\vspace":    true, "\\vspace*": true,
	// Float meta
	"\\caption": true, "\\caption*": true,
	// File inclusion
	"\\input":   true,
	"\\include": true,
	"\\subfile": true,
	"\\import":  true,
	// Front / back matter
	"\\maketitle":         true,
	"\\tableofcontents":   true,
	"\\listoffigures":     true,
	"\\listoftables":      true,
	"\\printbibliography": true,
	"\\bibliography":      true,
	"\\appendix":          true,
	"\\frontmatter":       true,
	"\\mainmatter":        true,
	"\\backmatter":        true,
	// Preamble
	"\\documentclass":       true,
	"\\usepackage":          true,
	"\\title":               true,
	"\\author":              true,
	"\\date":                true,
	"\\newcommand":          true,
	"\\renewcommand":        true,
	"\\newenvironment":      true,
	"\\DeclareMathOperator": true,
	"\\theoremstyle":        true,
	"\\newtheorem":          true,
	// Tabular rules
	"\\hline":      true,
	"\\midrule":    true,
	"\\toprule":    true,
	"\\bottomrule": true,
	"\\cmidrule":   true,
}

func checkBlockOnNewline(path string, lines []string) []Diagnostic {
	mask := regionMask(lines)
	var diags []Diagnostic
	for li, line := range lines {
		for _, v := range findBlockViolations(line, mask[li]) {
			diags = append(diags, Diagnostic{
				File:    path,
				Line:    li + 1,
				Message: fmt.Sprintf("block-level token %s should start a new line (column %d)", v.tok, v.pos+1),
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
			splits[k] = v.pos
		}
		out = append(out, splitLineAt(body, comment, body[:leadingWS(body)], splits)...)
		changed = true
	}
	return out, changed
}

type blockViolation struct {
	pos int    // byte offset of the violating token in the line
	tok string // token text (with leading backslash)
}

// findBlockViolations returns the positions of block tokens that are not at
// the leading-whitespace column. The first block token on a line (if it sits
// at the indent) is allowed; subsequent ones are violations.
func findBlockViolations(line string, mask []regionKind) []blockViolation {
	var out []blockViolation
	leadEnd := leadingWS(line)
	i := leadEnd
	for i < len(line) {
		if i < len(mask) && mask[i] == regVerbatim {
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
		isBlock := blockTokens[tok]
		isEnv := tok == "\\begin" || tok == "\\end"
		if !isBlock && !isEnv {
			i += n
			continue
		}
		if i != leadEnd {
			out = append(out, blockViolation{pos: i, tok: tok})
		}
		// Advance past the token (and the env brace group, if applicable).
		i += n
		if isEnv && i < len(line) && line[i] == '{' {
			if end := strings.IndexByte(line[i:], '}'); end >= 0 {
				i += end + 1
			}
		}
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
