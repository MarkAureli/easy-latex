package pedantic

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

func init() {
	Register(Check{
		Name:   "sentence-terminator",
		Phase:  PhaseSource,
		Source: checkSentenceTerminator,
	})
}

// sentenceExemptLeading are leading tokens whose line is never checked for a
// terminal punctuation mark (titles, list items, structural commands, ...).
// Derived from blockTokens' leading entries, minus \caption (its argument is
// prose and IS checked), plus \label/\index (pure metadata, never prose).
var sentenceExemptLeading = buildSentenceExemptLeading()

func buildSentenceExemptLeading() map[string]bool {
	out := make(map[string]bool, len(blockTokens)+2)
	for tok, kind := range blockTokens {
		if kind != blockLeading || tok == "\\caption" || tok == "\\caption*" {
			continue
		}
		out[tok] = true
	}
	out["\\label"] = true
	out["\\index"] = true
	return out
}

// tabularEnvs hold row/cell content, not prose, and are exempt line-by-line
// (the \begin/\end lines themselves are already exempt as block tokens).
var tabularEnvs = map[string]bool{
	"tabular": true, "tabular*": true, "tabularx": true,
	"longtable": true, "array": true,
}

// trailingDecoratorRe matches a trailing decoration (footnote, label,
// citation, ...) that doesn't itself carry the sentence's terminal
// punctuation; it's stripped so the real preceding terminator can be found.
var trailingDecoratorRe = regexp.MustCompile(`\\(?:footnotemark|footnote|label|index|marginpar|citeauthor|citeyear|nocite|citep|citet|cite|qedhere)\*?(?:\{[^{}]*\})?\s*$`)

var trailingClosers = map[rune]bool{
	')': true, ']': true, '"': true, '\'': true, '}': true,
	'’': true, '”': true,
}

var listEnvNames = map[string]bool{"itemize": true, "enumerate": true, "description": true}

func checkSentenceTerminator(path string, lines []string) []Diagnostic {
	mask := regionMask(lines)
	inTabular := tabularLines(lines, mask)
	var diags []Diagnostic
	for li, line := range lines {
		if inTabular[li] {
			continue
		}
		body := strings.TrimRight(line, " \t")
		if body == "" {
			continue
		}
		lastIdx := len(body) - 1
		if mask[li][lastIdx] != regText {
			continue // ends mid-math/mid-verbatim: handled as its own unit
		}
		lead := leadingWS(body)
		if lead < len(body) && body[lead] == '\\' {
			tok, _ := nextTokenAt(body, lead)
			if tok == "\\begin" || tok == "\\end" || sentenceExemptLeading[tok] {
				continue
			}
		}
		if hasTrailingLineBreakToken(body) {
			continue
		}
		core := stripTrailingDecorations(body)
		if core == "" {
			continue
		}
		if endsWithTerminator(core) {
			continue
		}
		if strings.HasSuffix(core, ":") && nextLineOpensListOrMath(lines, li+1) {
			continue
		}
		diags = append(diags, Diagnostic{
			File:    path,
			Line:    li + 1,
			Message: fmt.Sprintf("sentence should end with '.', '?', or '!' (column %d)", len(core)),
		})
	}
	return diags
}

// tabularLines reports, per line, whether that line's content sits strictly
// inside a tabular-like environment (row/cell content, not the \begin/\end
// lines themselves).
func tabularLines(lines []string, mask [][]regionKind) []bool {
	out := make([]bool, len(lines))
	depth := 0
	for li, line := range lines {
		out[li] = depth > 0
		i := 0
		for i < len(line) {
			if i < len(mask[li]) && mask[li][i] != regText {
				i++
				continue
			}
			if line[i] != '\\' {
				i++
				continue
			}
			if name, n := matchBeginEnd(line, i, "begin"); n > 0 {
				if tabularEnvs[name] {
					depth++
				}
				i += n
				continue
			}
			if name, n := matchBeginEnd(line, i, "end"); n > 0 {
				if tabularEnvs[name] && depth > 0 {
					depth--
				}
				i += n
				continue
			}
			i++
		}
	}
	return out
}

// hasTrailingLineBreakToken reports whether body ends with a forced-break
// token (\\ or \newline); the sentence continues past it, so no terminal
// punctuation is required on this line.
func hasTrailingLineBreakToken(body string) bool {
	trimmed := strings.TrimRight(body, " \t")
	if strings.HasSuffix(trimmed, "\\\\") {
		return true
	}
	if strings.HasSuffix(trimmed, "\\newline") {
		i := len(trimmed) - len("\\newline")
		if i == 0 || !isLetter(trimmed[i-1]) {
			return true
		}
	}
	return false
}

// stripTrailingDecorations repeatedly strips trailing decoration commands
// (\footnote{...}, \label{...}, \cite{...}, ...) and generic closing
// brackets/quotes/braces from s, exposing the text that should carry the
// sentence's real terminal punctuation. A decoration is only stripped as a
// trailing decoration when something remains before it — otherwise it *is*
// the line's whole content (e.g. a standalone \footnote{...} or
// \caption{...}) and stripping falls through to the generic closer path,
// which descends into its argument instead.
func stripTrailingDecorations(s string) string {
	for {
		s = strings.TrimRight(s, " \t")
		if loc := trailingDecoratorRe.FindStringIndex(s); loc != nil {
			if candidate := strings.TrimRight(s[:loc[0]], " \t"); candidate != "" {
				s = candidate
				continue
			}
		}
		if s == "" {
			return s
		}
		r, size := utf8.DecodeLastRuneInString(s)
		if trailingClosers[r] {
			s = s[:len(s)-size]
			continue
		}
		return s
	}
}

func endsWithTerminator(s string) bool {
	if s == "" {
		return false
	}
	return strings.HasSuffix(s, ".") || strings.HasSuffix(s, "?") ||
		strings.HasSuffix(s, "!") || strings.HasSuffix(s, "…") ||
		strings.HasSuffix(s, "\\ldots")
}

// nextLineOpensListOrMath looks ahead (skipping blank lines, bounded) for a
// line opening a list or display-math environment, which licenses a `:`
// lead-in terminator on the current line.
func nextLineOpensListOrMath(lines []string, from int) bool {
	for i := from; i < len(lines) && i < from+3; i++ {
		body := strings.TrimSpace(lines[i])
		if body == "" {
			continue
		}
		if strings.HasPrefix(body, "\\[") {
			return true
		}
		if name, n := matchBeginEnd(body, 0, "begin"); n > 0 {
			return listEnvNames[name] || displayMathEnvs[name]
		}
		return false
	}
	return false
}
