package texscan

import (
	"regexp"
	"strings"
)

// lengthLiteralRe matches TeX length literals: an optional sign, digits with
// optional decimal part, immediately followed by a known unit. Word boundary
// after the unit prevents matching prefixes of longer identifiers (`5cmore`).
var lengthLiteralRe = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:pt|pc|in|bp|cm|mm|dd|cc|nd|nc|sp|em|ex|px|mu|filll|fill|fil)\b`)

// blankLengthLiterals replaces every length literal in buf with spaces, in
// place. Length-preserving so column offsets stay aligned.
func blankLengthLiterals(buf []byte) {
	for _, m := range lengthLiteralRe.FindAllIndex(buf, -1) {
		for i := m[0]; i < m[1]; i++ {
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

// proseMathEnvs are environments whose body is treated as math.
var proseMathEnvs = map[string]bool{
	"equation":    true,
	"equation*":   true,
	"align":       true,
	"align*":      true,
	"gather":      true,
	"gather*":     true,
	"multline":    true,
	"multline*":   true,
	"eqnarray":    true,
	"eqnarray*":   true,
	"displaymath": true,
	"math":        true,
	"flalign":     true,
	"flalign*":    true,
	"alignat":     true,
	"alignat*":    true,
}

// proseVerbatimEnvs are environments whose body is opaque verbatim text.
var proseVerbatimEnvs = map[string]bool{
	"verbatim":   true,
	"Verbatim":   true,
	"BVerbatim":  true,
	"LVerbatim":  true,
	"lstlisting": true,
	"minted":     true,
	"comment":    true,
	"alltt":      true,
}

// ProseRuns extracts prose text from tex content. Returns one ProseRun per
// source line that contains at least one prose byte. Skipped (blanked):
// comments (via StripComment), math regions ($, $$, \(, \[, math envs),
// verbatim envs, macro names (`\foo`, `\foo*`), braces/brackets, and the first
// brace-balanced argument of any macro in ignoreMacros (plus its leading
// optional `[...]` args).
func ProseRuns(file, content string, ignoreMacros map[string]bool) []ProseRun {
	rawLines := strings.Split(content, "\n")
	lines := make([]string, len(rawLines))
	for i, l := range rawLines {
		lines[i] = expandAccents(StripComment(l))
	}

	type state int
	const (
		stText state = iota
		stInlineMath
		stDisplayMath
		stMathParen  // \( ... \)
		stMathBrack  // \[ ... \]
		stMathEnv    // \begin{<envMath>} ... \end{...}
		stVerbEnv    // verbatim env
		stIgnoreArg  // skipping first {...} of an ignored macro
	)

	st := stText
	envName := ""    // for stMathEnv / stVerbEnv
	braceDepth := 0  // for stIgnoreArg
	awaitingArg := false // ignored macro consumed; expecting `[opt]*` then `{`
	awaitOptDepth := 0   // bracket depth while consuming `[opt]` args

	out := make([]ProseRun, 0, len(lines))

	for li, line := range lines {
		buf := []byte(line)
		blank := func(i int) { buf[i] = ' ' }

		i := 0
		for i < len(line) {
			c := line[i]

			switch st {
			case stText:
				if awaitingArg {
					// Skip whitespace, optional `[...]` groups, then expect `{`.
					if c == ' ' || c == '\t' {
						i++
						continue
					}
					if c == '[' {
						blank(i)
						awaitOptDepth = 1
						i++
						for i < len(line) && awaitOptDepth > 0 {
							switch line[i] {
							case '[':
								awaitOptDepth++
							case ']':
								awaitOptDepth--
							}
							blank(i)
							i++
						}
						if awaitOptDepth > 0 {
							// Unclosed `[`; abandon arg-skip.
							awaitingArg = false
						}
						continue
					}
					if c == '{' {
						blank(i)
						st = stIgnoreArg
						braceDepth = 1
						awaitingArg = false
						i++
						continue
					}
					// Anything else: macro had no `{` arg; resume normal text.
					awaitingArg = false
					continue
				}

				// Escapes that aren't math/macro openers we care about: \$ \% \\ \{ \} \_ \&
				if c == '\\' && i+1 < len(line) {
					nx := line[i+1]
					if nx == '$' || nx == '%' || nx == '&' || nx == '_' || nx == '#' {
						blank(i)
						blank(i + 1)
						i += 2
						continue
					}
					if nx == '\\' {
						// `\\` line break — blank.
						blank(i)
						blank(i + 1)
						i += 2
						continue
					}
					if nx == '{' || nx == '}' {
						// Literal brace — keep as prose-neutral; blank to be safe.
						blank(i)
						blank(i + 1)
						i += 2
						continue
					}
					if nx == '(' {
						blank(i)
						blank(i + 1)
						st = stMathParen
						i += 2
						continue
					}
					if nx == '[' {
						blank(i)
						blank(i + 1)
						st = stMathBrack
						i += 2
						continue
					}
					if name, n := matchBeginEnvName(line, i); n > 0 {
						for j := range n {
							blank(i + j)
						}
						i += n
						switch {
						case proseVerbatimEnvs[name]:
							st = stVerbEnv
							envName = name
						case proseMathEnvs[name]:
							st = stMathEnv
							envName = name
						}
						// non-math non-verbatim env: stay in text, but the
						// `\begin{name}` letters were already blanked.
						continue
					}
					if _, n := matchEndEnvName(line, i); n > 0 {
						// `\end{...}` of a non-math/non-verbatim env in text
						// state — just blank the tokens.
						for j := range n {
							blank(i + j)
						}
						i += n
						continue
					}
					// Generic `\macroName` (letters, possibly trailing `*`).
					name, n := readMacroName(line, i)
					if n > 0 {
						for j := range n {
							blank(i + j)
						}
						i += n
						if ignoreMacros[name] {
							awaitingArg = true
						}
						continue
					}
					// Lone backslash, non-letter follower we don't recognize.
					blank(i)
					i++
					continue
				}

				if c == '$' {
					blank(i)
					if i+1 < len(line) && line[i+1] == '$' {
						blank(i + 1)
						st = stDisplayMath
						i += 2
					} else {
						st = stInlineMath
						i++
					}
					continue
				}

				if c == '{' || c == '}' || c == '[' || c == ']' {
					blank(i)
					i++
					continue
				}

				// Prose byte: keep as-is.
				i++

			case stInlineMath:
				if c == '\\' && i+1 < len(line) {
					blank(i)
					blank(i + 1)
					i += 2
					continue
				}
				if c == '$' {
					blank(i)
					st = stText
					i++
					continue
				}
				blank(i)
				i++

			case stDisplayMath:
				if c == '\\' && i+1 < len(line) {
					blank(i)
					blank(i + 1)
					i += 2
					continue
				}
				if c == '$' && i+1 < len(line) && line[i+1] == '$' {
					blank(i)
					blank(i + 1)
					st = stText
					i += 2
					continue
				}
				blank(i)
				i++

			case stMathParen:
				if c == '\\' && i+1 < len(line) && line[i+1] == ')' {
					blank(i)
					blank(i + 1)
					st = stText
					i += 2
					continue
				}
				if c == '\\' && i+1 < len(line) {
					blank(i)
					blank(i + 1)
					i += 2
					continue
				}
				blank(i)
				i++

			case stMathBrack:
				if c == '\\' && i+1 < len(line) && line[i+1] == ']' {
					blank(i)
					blank(i + 1)
					st = stText
					i += 2
					continue
				}
				if c == '\\' && i+1 < len(line) {
					blank(i)
					blank(i + 1)
					i += 2
					continue
				}
				blank(i)
				i++

			case stMathEnv, stVerbEnv:
				if c == '\\' {
					if name, n := matchEndEnvName(line, i); n > 0 && name == envName {
						for j := range n {
							blank(i + j)
						}
						st = stText
						envName = ""
						i += n
						continue
					}
					if i+1 < len(line) {
						blank(i)
						blank(i + 1)
						i += 2
						continue
					}
				}
				blank(i)
				i++

			case stIgnoreArg:
				blank(i)
				if c == '\\' && i+1 < len(line) {
					blank(i + 1)
					i += 2
					continue
				}
				switch c {
				case '{':
					braceDepth++
				case '}':
					braceDepth--
					if braceDepth == 0 {
						st = stText
						i++
						continue
					}
				}
				i++
			}
		}

		blankLengthLiterals(buf)
		text := string(buf)
		if strings.TrimSpace(text) != "" {
			out = append(out, ProseRun{File: file, Line: li + 1, Text: text})
		}
	}
	return out
}

// accentChars are TeX accent prefixes that take a single letter argument:
// `\"a` (umlaut), `\'e` (acute), `` \`o `` (grave), `\~n` (tilde), `\=u`
// (macron), `\.s` (overdot), `\^a` (circumflex).
var accentChars = map[byte]bool{
	'"': true, '\'': true, '`': true, '~': true, '=': true, '.': true, '^': true,
}

// expandAccents performs a length-preserving substitution of TeX accent-macro
// patterns to letter padding so that hunspell sees a single compound token
// rather than fragments split by `\`/`{`/`}`. The substitution covers:
//
//	\"a       → 3 letters
//	\"{a}     → 5 letters
//	{\"a}     → 5 letters
//	{\"{a}}   → 7 letters
//
// The "letter" written is the inside character itself, ASCII-cased. Anything
// not matching is left untouched. Length is always preserved so column offsets
// in downstream prose runs remain accurate.
func expandAccents(line string) string {
	b := []byte(line)
	i := 0
	for i < len(b) {
		if n := matchAccent(b, i); n > 0 {
			letter := accentLetter(b[i : i+n])
			for j := range n {
				b[i+j] = letter
			}
			i += n
			continue
		}
		i++
	}
	return string(b)
}

// matchAccent checks whether an accent pattern starts at b[i]. Returns its
// length (3, 5, or 7) on a match, else 0.
func matchAccent(b []byte, i int) int {
	// Forms starting with `{`.
	if b[i] == '{' {
		// {\<acc>{<L>}}
		if i+6 < len(b) && b[i+1] == '\\' && accentChars[b[i+2]] && b[i+3] == '{' && isLetter(b[i+4]) && b[i+5] == '}' && b[i+6] == '}' {
			return 7
		}
		// {\<acc><L>}
		if i+4 < len(b) && b[i+1] == '\\' && accentChars[b[i+2]] && isLetter(b[i+3]) && b[i+4] == '}' {
			return 5
		}
		return 0
	}
	// Forms starting with `\`.
	if b[i] == '\\' {
		// \<acc>{<L>}
		if i+4 < len(b) && accentChars[b[i+1]] && b[i+2] == '{' && isLetter(b[i+3]) && b[i+4] == '}' {
			return 5
		}
		// \<acc><L>
		if i+2 < len(b) && accentChars[b[i+1]] && isLetter(b[i+2]) {
			return 3
		}
	}
	return 0
}

// accentLetter returns the ASCII letter inside an accent-macro pattern.
func accentLetter(seg []byte) byte {
	for _, c := range seg {
		if isLetter(c) {
			return c
		}
	}
	return 'a' // unreachable for well-formed input
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
