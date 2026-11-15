package texscan

import (
	"regexp"
	"strings"
)

// lengthLiteralRe matches TeX length literals: an optional sign, digits with
// optional decimal part, immediately followed by a known unit. Word boundary
// after the unit prevents matching prefixes of longer identifiers (`5cmore`).
var lengthLiteralRe = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:pt|pc|in|bp|cm|mm|dd|cc|nd|nc|sp|em|ex|px|mu|filll|fill|fil)\b`)

// ordinalAfterMathRe matches an English ordinal suffix (`st`, `nd`, `rd`, `th`)
// that immediately follows a math closer (`$`, `\)`, `\]`). Captures only the
// suffix so callers can blank just those bytes. Catches `$n$th`, `\(k\)st`,
// `\[m\]rd`, etc.
var ordinalAfterMathRe = regexp.MustCompile(`(?:\$|\\\)|\\\])(st|nd|rd|th)\b`)

// ordinalInSuperscriptRe matches an ordinal suffix inside the brace argument
// of `\textsuperscript`/`\textsubscript`. Captures the suffix so the inside
// of the braces can be blanked while the macro+braces remain blanked by the
// regular state machine.
var ordinalInSuperscriptRe = regexp.MustCompile(`\\text(?:super|sub)script\s*\{(st|nd|rd|th)\}`)

// blankLengthLiterals replaces every length literal in buf with spaces, in
// place. Length-preserving so column offsets stay aligned.
func blankLengthLiterals(buf []byte) {
	for _, m := range lengthLiteralRe.FindAllIndex(buf, -1) {
		for i := m[0]; i < m[1]; i++ {
			buf[i] = ' '
		}
	}
}

// blankOrdinalSuffixes blanks ordinal suffixes that follow math closers or
// appear inside `\textsuperscript{}`/`\textsubscript{}` braces. Operates on
// the original line for matching (so math markers are visible) and rewrites
// buf at the captured suffix positions only.
func blankOrdinalSuffixes(line string, buf []byte) {
	for _, m := range ordinalAfterMathRe.FindAllStringSubmatchIndex(line, -1) {
		for i := m[2]; i < m[3]; i++ {
			buf[i] = ' '
		}
	}
	for _, m := range ordinalInSuperscriptRe.FindAllStringSubmatchIndex(line, -1) {
		for i := m[2]; i < m[3]; i++ {
			buf[i] = ' '
		}
	}
}

// ProseRun represents one source line projected to prose-only text. Non-prose
// bytes (macro names, braces, math content, verbatim content, ignored-macro
// args) are replaced by single spaces so byte offsets in Text map 1:1 to
// columns in the original (comment-stripped) line.
type ProseRun struct {
	File string
	Line int    // 1-based source line number
	Text string // same length as the source line; non-prose bytes blanked
}

// ProseRuns extracts prose text from tex content: one ProseRun per source
// line that contains at least one prose byte (see Classify). Comments, math,
// verbatim, markup and opaque bytes (arguments of unknown or project-defined
// commands, options, definitions, graphics code) are blanked, as are TeX
// length literals and ordinal suffixes after math.
func ProseRuns(file, content string) []ProseRun {
	rawLines := strings.Split(content, "\n")
	lines := make([]string, len(rawLines))
	for i, l := range rawLines {
		lines[i] = StripComment(l)
	}
	out := make([]ProseRun, 0, len(lines))
	for li, cs := range Classify(lines) {
		line := lines[li]
		buf := []byte(line)
		for i, c := range cs {
			if !c.IsProse() {
				buf[i] = ' '
			}
		}
		blankLengthLiterals(buf)
		blankOrdinalSuffixes(line, buf)
		if text := string(buf); strings.TrimSpace(text) != "" {
			out = append(out, ProseRun{File: file, Line: li + 1, Text: text})
		}
	}
	return out
}

// readMacroName reads `\<macroChars>(*?)` starting at i (where line[i]=='\\').
// macroChars are ASCII letters plus `@` (TeX-internal commands like
// `\@oddfoot` under `\makeatletter`). Returns name (without backslash, without
// trailing `*`) and total length consumed. Returns ("", 0) if no macro chars.
func readMacroName(line string, i int) (string, int) {
	if i >= len(line) || line[i] != '\\' {
		return "", 0
	}
	j := i + 1
	for j < len(line) && isMacroChar(line[j]) {
		j++
	}
	if j == i+1 {
		return "", 0
	}
	name := line[i+1 : j]
	if j < len(line) && line[j] == '*' {
		j++
	}
	return name, j - i
}

// isMacroChar reports whether c may appear in a TeX macro name.
func isMacroChar(c byte) bool {
	return isLetter(c) || c == '@'
}

// matchBeginEnvName matches `\begin{<name>}` at i and returns (name, length).
func matchBeginEnvName(line string, i int) (string, int) {
	const prefix = "\\begin{"
	if !strings.HasPrefix(line[i:], prefix) {
		return "", 0
	}
	rest := line[i+len(prefix):]
	end := strings.IndexByte(rest, '}')
	if end < 0 {
		return "", 0
	}
	return rest[:end], len(prefix) + end + 1
}

// matchEndEnvName matches `\end{<name>}` at i.
func matchEndEnvName(line string, i int) (string, int) {
	const prefix = "\\end{"
	if !strings.HasPrefix(line[i:], prefix) {
		return "", 0
	}
	rest := line[i+len(prefix):]
	end := strings.IndexByte(rest, '}')
	if end < 0 {
		return "", 0
	}
	return rest[:end], len(prefix) + end + 1
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
