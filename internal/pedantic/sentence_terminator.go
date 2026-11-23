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

// trailingDecoratorRe matches a trailing decoration (footnote, label,
// citation, ...) that doesn't itself carry the sentence's terminal
// punctuation; it's stripped so the real preceding terminator can be found.
var trailingDecoratorRe = regexp.MustCompile(`\\(?:footnotemark|footnote|label|index|marginpar|citeauthor|citeyear|nocite|citep|citet|cite|qedhere)\*?(?:\{[^{}]*\})?\s*$`)

var trailingClosers = map[rune]bool{
	')': true, ']': true, '"': true, '\'': true, '}': true,
	'’': true, '”': true,
}

var listEnvNames = map[string]bool{"itemize": true, "enumerate": true, "description": true}

func checkSentenceTerminator(_ *Context, path string, lines []string) []Diagnostic {
	info := texscan.Analyze(lines)
	outside := outsideBody(lines, info)
	var diags []Diagnostic
	for li, line := range lines {
		in := info[li]
		start := texscan.ClassText // context at the start of the line
		if li > 0 {
			start = info[li-1].End
		}
		// Only lines starting in running or standalone prose (or in inline math
		// continued from the previous line) and ending in running or
		// standalone prose can end a sentence: headings (also multi-line ones), table cells,
		// opaque arguments, code, math and verbatim contexts are exempt, as
		// are lines whose group (e.g. a \footnote{) continues on the next line.
		if outside[li] || in.OpensGroup || !(sentenceContext(start) || start == texscan.ClassMath) || !sentenceContext(in.End) {
			continue
		}
		body := strings.TrimRight(line, " \t")
		if body == "" {
			continue
		}
		lastIdx := len(body) - 1
		if in.Math[lastIdx] || in.Classes[lastIdx] == texscan.ClassVerbatim {
			continue // ends mid-math/mid-verbatim: handled as its own unit
		}
		lead := leadingWS(body)
		if lead < len(body) && body[lead] == '\\' {
			tok, _ := nextTokenAt(body, lead)
			if tok == "\\begin" || tok == "\\end" || sentenceExemptLeading[tok] {
				continue
			}
		}
		if hasTrailingLineBreakToken(body) || isCommandOnly(in, body) || isBracedLine(body[lead:]) {
			continue
		}
		core := stripTrailingDecorations(body)
		if core == "" || endsWithTerminator(core) {
			continue
		}
		if nextLineContinuesSentence(lines, info, li+1) {
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

// isBracedLine reports whether s is a single {…} group, e.g. a sub-figure
// label `{\textbf{(a)} $K_4$}`: set-off formatting, not a sentence.
func isBracedLine(s string) bool {
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i == len(s)-1
			}
		}
	}
	return false
}

// sentenceContext reports whether a line ending in context c can end a
// sentence.
func sentenceContext(c texscan.Class) bool {
	return c == texscan.ClassText || c == texscan.ClassStandalone
}

// isCommandOnly reports whether line holds no running/standalone prose and no
// math, only markup, opaque arguments and heading-like prose — e.g.
// `\hypersetup{hidelinks}`, `\def\arraystretch{1.5}`, `\cite[Lemma 2]{key}`,
// `{\fontsize{10}{12}\selectfont` or `\affiliation{Some University}`.
func isCommandOnly(in texscan.LineInfo, line string) bool {
	saw := false
	for i := 0; i < len(line) && i < len(in.Classes); i++ {
		c := in.Classes[i]
		if line[i] == ' ' || line[i] == '\t' {
			continue
		}
		if sentenceContext(c) || in.Math[i] || c == texscan.ClassMath {
			return false
		}
		saw = true
	}
	return saw
}

// outsideBody reports, per line, whether it lies outside the document body:
// before \begin{document} / after \end{document} when the file has one, or
// the whole file when it has none but loads packages (a preamble fragment);
// other included files are all body.
func outsideBody(lines []string, info []texscan.LineInfo) []bool {
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
	for li, line := range lines {
		out[li] = outside
		for i := 0; i < len(line); i++ {
			if line[i] != '\\' || info[li].Math[i] || info[li].Classes[i] == texscan.ClassVerbatim {
				continue
			}
			if name, n := matchBeginEnd(line, i, "begin"); n > 0 && name == "document" {
				outside = false
			} else if name, n := matchBeginEnd(line, i, "end"); n > 0 && name == "document" {
				outside = true
			}
		}
	}
	return out
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
// token (\\, \\*, \\[len] or \newline); the sentence continues past it, so no terminal
// punctuation is required on this line.
func hasTrailingLineBreakToken(body string) bool {
	trimmed := strings.TrimRight(body, " \t")
	// \\[1cm], \\*: the break's own arguments.
	if strings.HasSuffix(trimmed, "]") {
		if j := strings.LastIndexByte(trimmed, '['); j > 0 {
			trimmed = strings.TrimRight(trimmed[:j], " \t")
		}
	}
	trimmed = strings.TrimSuffix(trimmed, "*")
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
func nextLineContinuesSentence(lines []string, info []texscan.LineInfo, from int) bool {
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
		if isCommandOnly(info[i], lines[i]) {
			continue
		}
		return false
	}
	return false
}
