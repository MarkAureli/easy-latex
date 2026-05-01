package pedantic

import (
	"fmt"
	"strings"
)

func init() {
	Register(Check{
		Name:   "math-bare-word",
		Phase:  PhaseSource,
		Source: checkMathBareWord,
	})
}

// isArgSkipCmd reports whether the braced argument of cmd should be skipped
// entirely and not scanned for bare words:
//   - \text family: \text, \textbf, \textrm, \textit, \textsf, \texttt, …
//   - explicit text boxes: \mbox, \hbox, \intertext
//   - upright / sans / typewriter math fonts: \mathrm, \mathsf, \mathtt, \mathit
//   - identifier arguments: \label{…}, \begin{…}, \end{…}
func isArgSkipCmd(name string) bool {
	switch name {
	case "mbox", "hbox", "intertext",
		"mathrm", "mathsf", "mathtt", "mathit",
		"label", "begin", "end":
		return true
	}
	return strings.HasPrefix(name, "text")
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// checkMathBareWord flags sequences of 2+ consecutive ASCII letters in math
// mode that are neither a LaTeX command (preceded by \) nor inside a text/font
// wrapper or identifier argument (\text{…}, \mathrm{…}, \mbox{…}, \label{…},
// \begin{…}, \end{…}, …).
func checkMathBareWord(path string, lines []string) []Diagnostic {
	mask := regionMask(lines)
	var diags []Diagnostic

	for li, line := range lines {
		m := mask[li]
		i := 0
		for i < len(line) {
			if i >= len(m) || m[i] != regMath {
				i++
				continue
			}

			// Command: skip its name; if it is a text wrapper also skip {…}.
			if line[i] == '\\' {
				i++ // consume backslash
				cmdStart := i
				for i < len(line) && isASCIILetter(line[i]) {
					i++
				}
				if isArgSkipCmd(line[cmdStart:i]) && i < len(line) && line[i] == '{' {
					depth := 1
					i++ // consume opening {
					for i < len(line) && depth > 0 {
						switch line[i] {
						case '{':
							depth++
						case '}':
							depth--
						}
						i++
					}
				}
				continue
			}

			// Letter sequence: flag runs of 2+.
			if isASCIILetter(line[i]) {
				start := i
				for i < len(line) && i < len(m) && m[i] == regMath && isASCIILetter(line[i]) {
					i++
				}
				if i-start >= 2 {
					diags = append(diags, Diagnostic{
						File:    path,
						Line:    li + 1,
						Message: fmt.Sprintf("bare word %q in math mode; use \\text{...} or a macro", line[start:i]),
					})
				}
				continue
			}

			i++
		}
	}
	return diags
}
