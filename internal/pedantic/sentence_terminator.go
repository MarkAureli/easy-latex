package pedantic

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/MarkAureli/easy-latex/internal/texscan"
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

// proseCmds take a prose argument. A line made up solely of other commands
// (and their arguments) is exempt, and so is the content of a multi-line brace
// group opened by any other command (\author{, \tikzset{, \newcommand{\x}{, ...).
var proseCmds = map[string]bool{
	"\\caption": true, "\\caption*": true, "\\footnote": true, "\\thanks": true,
	"\\marginpar": true, "\\emph": true, "\\text": true, "\\textbf": true,
	"\\textit": true, "\\textsl": true, "\\textsc": true, "\\textup": true,
	"\\textmd": true, "\\textrm": true, "\\textsf": true, "\\texttt": true,
	"\\underline": true,
}

// theoremEnvNames are theorem-like environments a sentence may lead into
// ("..., we obtain" + \begin{theorem}).
var theoremEnvNames = map[string]bool{
	"theorem": true, "lemma": true, "corollary": true, "proposition": true,
	"definition": true, "claim": true, "conjecture": true, "remark": true,
	"example": true, "assumption": true,
}

// lookaheadTransparentEnvs are layout wrappers the lookahead looks past
// ("we obtain" + \begin{widetext} + \begin{align}).
var lookaheadTransparentEnvs = map[string]bool{"widetext": true}

// preambleOnlyTokens can only appear in a preamble; a file containing one but
// no \begin{document} is an \input-ed preamble fragment (packages.tex, ...).
var preambleOnlyTokens = []string{"\\usepackage", "\\RequirePackage", "\\documentclass"}

// nonProseEnvs hold row/cell, drawing, pseudocode or reference-list content,
// not prose, and are exempt line-by-line (the \begin/\end lines themselves are
// already exempt as block tokens).
var nonProseEnvs = map[string]bool{
	"tabular": true, "tabular*": true, "tabularx": true,
	"longtable": true, "array": true,
	"tikzpicture": true, "quantikz": true,
	"algorithm": true, "algorithm*": true, "algorithmic": true,
	"thebibliography": true, "IEEEkeywords": true,
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
	nonProse := nonProseLines(lines, mask)
	groups := braceGroupLines(lines, mask)
	var diags []Diagnostic
	for li, line := range lines {
		if nonProse[li] || groups[li].inExempt || groups[li].leftOpen {
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
		if hasTrailingLineBreakToken(body) || isCommandOnly(body[lead:]) {
			continue
		}
		core := stripTrailingDecorations(body)
		if core == "" || endsWithTerminator(core) {
			continue
		}
		if nextLineContinuesSentence(lines, li+1) {
			continue // sentence continues through the display/list/theorem
		}
		diags = append(diags, Diagnostic{
			File:    path,
			Line:    li + 1,
			Message: fmt.Sprintf("sentence should end with '.', '?', '!', or ':' (column %d)", len(core)),
		})
	}
	return diags
}

// nonProseLines reports, per line, whether it lies outside the document body
// or strictly inside a nonProseEnvs environment. Outside the body means before
// \begin{document} / after \end{document} when the file has one, or the whole
// file when it has none but loads packages (a preamble fragment); other
// included files are all body.
func nonProseLines(lines []string, mask [][]regionKind) []bool {
	out := make([]bool, len(lines))
	outside := false
	for _, line := range lines {
		if strings.Contains(line, "\\begin{document}") {
			outside = true
			break
		}
		for _, tok := range preambleOnlyTokens {
			if strings.Contains(line, tok) {
				outside = true
			}
		}
	}
	if outside && !containsLine(lines, "\\begin{document}") {
		return fillTrue(out)
	}
	depth := 0
	for li, line := range lines {
		out[li] = outside || depth > 0
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
				if name == "document" {
					outside = false
				} else if nonProseEnvs[name] {
					depth++
				}
				i += n
				continue
			}
			if name, n := matchBeginEnd(line, i, "end"); n > 0 {
				if name == "document" {
					outside = true
				} else if nonProseEnvs[name] && depth > 0 {
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

// braceLineInfo describes a line's position relative to multi-line brace
// groups.
type braceLineInfo struct {
	inExempt bool // inside a group, opened on an earlier line, whose command is non-prose
	leftOpen bool // opens a group that is still open at line end (content continues)
}

type braceGroup struct {
	cmd    string
	exempt bool
	line   int
	option bool // `[...]` group rather than `{...}`
}

func (g braceGroup) closer() byte {
	if g.option {
		return ']'
	}
	return '}'
}

// braceGroupLines tracks `{...}` groups across lines (text regions only,
// escaped braces ignored), plus `[...]` option groups left open at line end.
// A brace group's command is the control word directly before it, looking
// past `*`, `[...]` optional args and earlier `{...}` args (so
// `\newcommand{\x}{` belongs to \newcommand). A group is exempt when it, or
// any enclosing group, belongs to a non-prose command; anonymous `{` groups
// inherit from their parent. Multi-line option groups are always exempt.
func braceGroupLines(lines []string, mask [][]regionKind) []braceLineInfo {
	out := make([]braceLineInfo, len(lines))
	var stack []braceGroup
	for li, line := range lines {
		out[li].inExempt = len(stack) > 0 && stack[len(stack)-1].exempt
		pending := ""
		i := 0
		for i < len(line) {
			if i < len(mask[li]) && mask[li][i] != regText {
				i++
				continue
			}
			parentExempt := len(stack) > 0 && stack[len(stack)-1].exempt
			switch c := line[i]; c {
			case '\\':
				tok, n := nextTokenAt(line, i)
				if n == 0 {
					n = 1
				}
				if len(tok) > 1 && isLetter(tok[1]) {
					pending = tok
				} else {
					pending = ""
				}
				i += n
				continue
			case '{':
				exempt := parentExempt || (pending != "" && !proseCmds[pending])
				stack = append(stack, braceGroup{cmd: pending, exempt: exempt, line: li})
				pending = ""
			case '}', ']':
				if len(stack) > 0 && stack[len(stack)-1].closer() == c {
					pending = stack[len(stack)-1].cmd
					stack = stack[:len(stack)-1]
				}
			case '[':
				if pending != "" {
					if j := strings.IndexByte(line[i:], ']'); j >= 0 {
						i += j + 1
						continue
					}
					stack = append(stack, braceGroup{cmd: pending, exempt: true, line: li, option: true})
					pending = ""
				}
			case '*', ' ', '\t':
			default:
				pending = ""
			}
			i++
		}
		for _, g := range stack {
			if g.line == li {
				out[li].leftOpen = true
				break
			}
		}
	}
	return out
}

// isCommandOnly reports whether s consists solely of control sequences and
// their `*`/`[...]`/`{...}` arguments (plus whitespace), with a first command
// that is not a proseCmds entry — e.g. `\hypersetup{hidelinks}`,
// `\def\arraystretch{1.5}`, `\cite[Lemma 2]{key}`, or a bare `[1em]`.
func isCommandOnly(s string) bool {
	first := true
	sawAny := false
	for i := 0; i < len(s); {
		switch s[i] {
		case ' ', '\t', '*':
			i++
		case '\\':
			tok, n := nextTokenAt(s, i)
			if n == 0 {
				return false
			}
			if first && proseCmds[tok] {
				return false
			}
			first, sawAny = false, true
			i += n
		case '{', '[':
			n := matchingGroupLen(s, i)
			if n == 0 {
				return false
			}
			sawAny = true
			i += n
		default:
			return false
		}
	}
	return sawAny
}

// matchingGroupLen returns the byte length of the balanced group opening at
// s[i] (`{...}` or `[...]`), or 0 if it is not closed on this line.
func matchingGroupLen(s string, i int) int {
	open, close := s[i], byte('}')
	if open == '[' {
		close = ']'
	}
	depth := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return j - i + 1
			}
		}
	}
	return 0
}

func containsLine(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

func fillTrue(b []bool) []bool {
	for i := range b {
		b[i] = true
	}
	return b
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
		strings.HasSuffix(s, ":") ||
		strings.HasSuffix(s, "!") || strings.HasSuffix(s, "…") ||
		strings.HasSuffix(s, "\\ldots")
}

// nextLineContinuesSentence looks ahead (skipping blank and command-only
// lines such as \vspace or \label, and lookaheadTransparentEnvs openers such
// as \begin{widetext}, bounded) for a line opening a list, display math or a
// theorem-like environment. The current line's sentence then
// continues through that block, so it needs no terminator of its own.
func nextLineContinuesSentence(lines []string, from int) bool {
	for i := from; i < len(lines) && i < from+5; i++ {
		body := strings.TrimSpace(lines[i])
		if body == "" {
			continue
		}
		if strings.HasPrefix(body, "\\[") || strings.HasPrefix(body, "$$") {
			return true
		}
		if name, n := matchBeginEnd(body, 0, "begin"); n > 0 {
			if lookaheadTransparentEnvs[name] {
				continue
			}
			return listEnvNames[name] || texscan.IsMathEnv(name) ||
				strings.HasSuffix(name, "subequations") ||
				theoremEnvNames[strings.ToLower(strings.TrimSuffix(name, "*"))]
		}
		if isCommandOnly(strings.TrimLeft(body, "{ \t")) {
			continue
		}
		return false
	}
	return false
}
