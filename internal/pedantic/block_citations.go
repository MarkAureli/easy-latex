package pedantic

import (
	"fmt"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func init() {
	Register(Check{
		Name:   "no-block-citations",
		Phase:  PhaseSource,
		Source: checkBlockCitations,
	})
}

func checkBlockCitations(path string, lines []string) []Diagnostic {
	var diags []Diagnostic
	for i, line := range lines {
		lineNo := i + 1
		matches := texscan.ReCiteCall.FindAllStringSubmatchIndex(line, -1)

		// Check: single cite with multiple comma-separated keys
		for _, m := range matches {
			keysStr := line[m[2]:m[3]]
			if strings.Contains(keysStr, ",") {
				diags = append(diags, Diagnostic{
					File:    path,
					Line:    lineNo,
					Message: fmt.Sprintf("multiple keys in single cite command: %s", line[m[0]:m[1]]),
				})
			}
		}

		// Check: adjacent cite commands (separated only by whitespace or ~)
		for j := 0; j < len(matches)-1; j++ {
			between := line[matches[j][1]:matches[j+1][0]]
			trimmed := strings.TrimFunc(between, func(r rune) bool {
				return r == ' ' || r == '\t' || r == '~'
			})
			if trimmed == "" {
				diags = append(diags, Diagnostic{
					File:    path,
					Line:    lineNo,
					Message: fmt.Sprintf("adjacent cite commands: %s", line[matches[j][0]:matches[j+1][1]]),
				})
			}
		}
	}
	return diags
}
