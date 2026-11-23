package pedantic

import (
	"fmt"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func init() {
	Register(Check{
		Name:   "math-bare-word",
		Phase:  PhaseSource,
		Source: checkMathBareWord,
	})
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// checkMathBareWord flags runs of 2+ consecutive ASCII letters in math (see
// texscan.Classify). Command names are markup, and the arguments of text and
// font wrappers, labels and lengths (\text{…}, \mathrm{…}, \label{…}, …) are
// not math, so they are never flagged.
func checkMathBareWord(_ *Context, path string, lines []string) []Diagnostic {
	var diags []Diagnostic
	for li, cs := range texscan.Classify(lines) {
		line := lines[li]
		for i := 0; i < len(line); {
			if cs[i] != texscan.ClassMath || !isASCIILetter(line[i]) {
				i++
				continue
			}
			start := i
			for i < len(line) && cs[i] == texscan.ClassMath && isASCIILetter(line[i]) {
				i++
			}
			if i-start >= 2 {
				diags = append(diags, Diagnostic{
					File:    path,
					Line:    li + 1,
					Message: fmt.Sprintf("bare word %q in math mode", line[start:i]),
				})
			}
		}
	}
	return diags
}
