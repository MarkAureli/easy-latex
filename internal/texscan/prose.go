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
// Also includes graphics environments whose body is a graphics specification
// language (TikZ/PGF/CircuiTikZ/PSTricks/forest), not prose.
var proseVerbatimEnvs = map[string]bool{
	"verbatim":     true,
	"Verbatim":     true,
	"BVerbatim":    true,
	"LVerbatim":    true,
	"lstlisting":   true,
	"minted":       true,
	"comment":      true,
	"alltt":        true,
	"tikzpicture":  true,
	"pgfpicture":   true,
	"circuitikz":   true,
	"forest":       true,
	"pspicture":    true,
	"scope":        true,
	"axis":         true,
	"semilogxaxis": true,
	"semilogyaxis": true,
	"loglogaxis":   true,
}

// DefaultTextMacros is the built-in whitelist of macros whose brace arguments
// carry prose. The value is the number of leading brace arguments to skip
// (e.g. `\textcolor{red}{text}` skips the colour name, keeps `text`); `[opt]`
// arguments are always skipped. Every macro NOT in the effective whitelist has
// all of its `[...]` / `{...}` arguments skipped (see ProseRuns).
var DefaultTextMacros = map[string]int{
	// Font / emphasis
	"emph": 0, "textbf": 0, "textit": 0, "textrm": 0, "textsf": 0, "texttt": 0,
	"textsc": 0, "textsl": 0, "textup": 0, "textmd": 0, "textnormal": 0,
	"textsuperscript": 0, "textsubscript": 0, "underline": 0, "uline": 0,
	"sout": 0, "hl": 0, "highlight": 0, "enquote": 0,
	// Boxes
	"mbox": 0, "hbox": 0, "fbox": 0, "makebox": 0, "framebox": 0,
	"parbox": 1, "raisebox": 1, "scalebox": 1, "rotatebox": 1,
	"colorbox": 1, "fcolorbox": 2, "resizebox": 2,
	// Colour / links / language
	"textcolor": 1, "href": 1, "hyperlink": 1, "hypertarget": 1,
	"foreignlanguage": 1,
	// Structure
	"part": 0, "chapter": 0, "section": 0, "subsection": 0, "subsubsection": 0,
	"paragraph": 0, "subparagraph": 0, "caption": 0, "captionof": 1,
	"footnote": 0, "footnotetext": 0, "marginpar": 0, "item": 0, "bibitem": 1,
	// Front matter
	"title": 0, "subtitle": 0, "author": 0, "date": 0, "thanks": 0,
	"institute": 0, "affiliation": 0, "keywords": 0,
	// Tables
	"multicolumn": 2, "multirow": 2,
}

// defMacros introduce a new command name (`\newcommand\foo`, `\def\foo#1`).
// The unbraced name that follows is part of the skipped definition, so it must
// not be scanned as a macro of its own (it is whitelisted by DefinedMacros,
// which would otherwise expose the body as prose). The bool is true for the
// TeX primitives that take `#1#2…` parameter tokens before the body.
var defMacros = map[string]bool{
	"newcommand": false, "renewcommand": false, "providecommand": false,
	"DeclareRobustCommand": false, "NewDocumentCommand": false,
	"RenewDocumentCommand": false, "ProvideDocumentCommand": false,
	"DeclareDocumentCommand": false,
	"def":                    true, "gdef": true, "edef": true, "xdef": true,
}

var definedMacroRe = regexp.MustCompile(`\\(?:(?:re)?newcommand|providecommand|DeclareRobustCommand|(?:New|Renew|Provide|Declare)DocumentCommand|[gex]?def)\*?\s*\{?\s*\\([A-Za-z@]+)`)

// DefinedMacros returns the names of commands defined in content via
// `\newcommand`, `\renewcommand`, `\providecommand`, `\DeclareRobustCommand`,
// `\NewDocumentCommand` (and friends) or `\def`-style primitives. Comments are
// stripped first. Order of first appearance, duplicates removed.
func DefinedMacros(content string) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range strings.Split(content, "\n") {
		for _, m := range definedMacroRe.FindAllStringSubmatch(StripComment(l), -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				out = append(out, m[1])
			}
		}
	}
	return out
}

// ProseRuns extracts prose text from tex content. Returns one ProseRun per
// source line that contains at least one prose byte. Skipped (blanked):
// comments (via StripComment), math regions ($, $$, \(, \[, math envs),
// verbatim envs, macro names (`\foo`, `\foo*`), braces/brackets, and the
// arguments of every macro not in textMacros: all consecutive `[...]` / `{...}`
// groups (at any position) are skipped. Macros in textMacros keep their brace
// arguments as prose, except the first textMacros[name] ones; their `[opt]`
// arguments are skipped.
func ProseRuns(file, content string, textMacros map[string]int) []ProseRun {
	rawLines := strings.Split(content, "\n")
	lines := make([]string, len(rawLines))
	for i, l := range rawLines {
		lines[i] = StripComment(l)
	}

	type state int
	const (
		stText state = iota
		stInlineMath
		stDisplayMath
		stMathParen // \( ... \)
		stMathBrack // \[ ... \]
		stMathEnv   // \begin{<envMath>} ... \end{...}
		stVerbEnv   // verbatim env
		stIgnoreArg // skipping first {...} of an ignored macro
	)

	st := stText
	envName := ""            // for stMathEnv / stVerbEnv
	braceDepth := 0          // for stIgnoreArg
	awaitingArg := false     // arg-skipping macro consumed; expecting `[opt]*` then `{`
	awaitOptDepth := 0       // bracket depth while consuming `[opt]` args
	remainingIgnoreArgs := 0 // further {...} args to skip after the current one
	skipAllArgs := false     // non-text macro: keep skipping groups until a non-group token
	defName := false         // definition macro consumed; unbraced `\name` may follow
	defParams := false       // ...and `#1#2` parameter tokens after that name

	out := make([]ProseRun, 0, len(lines))

	for li, line := range lines {
		buf := []byte(line)
		blank := func(i int) { buf[i] = ' ' }
		if strings.TrimSpace(line) == "" {
			// Paragraph break: pending argument skips cannot span it.
			awaitingArg, skipAllArgs, defName = false, false, false
		}

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
					if defName {
						defName = false
						if c == '\\' {
							// `\newcommand\foo` / `\def\foo#1`: the name belongs to
							// the skipped definition, not to the text.
							_, n := readMacroName(line, i)
							if n == 0 {
								n = 1
							}
							for j := range n {
								blank(i + j)
							}
							i += n
							for defParams && i+1 < len(line) && line[i] == '#' && line[i+1] >= '0' && line[i+1] <= '9' {
								blank(i)
								blank(i + 1)
								i += 2
							}
							continue
						}
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
							skipAllArgs = false
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
					// Anything else: no further `{` arg; resume normal text.
					awaitingArg = false
					skipAllArgs = false
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
						// Blank any immediately-following `[...]` env options.
						i = blankOptArgs(line, buf, i)
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
						// Blank any immediately-following `[...]` option args.
						i = blankOptArgs(line, buf, i)
						skip, isText := textMacros[name]
						switch {
						case !isText:
							awaitingArg = true
							skipAllArgs = true
							remainingIgnoreArgs = 0
							defParams, defName = defMacros[name]
						case skip > 0:
							awaitingArg = true
							skipAllArgs = false
							remainingIgnoreArgs = skip - 1
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
						if remainingIgnoreArgs > 0 {
							remainingIgnoreArgs--
							awaitingArg = true
						} else if skipAllArgs {
							awaitingArg = true
						}
						i++
						continue
					}
				}
				i++
			}
		}

		blankLengthLiterals(buf)
		blankOrdinalSuffixes(line, buf)
		text := string(buf)
		if strings.TrimSpace(text) != "" {
			out = append(out, ProseRun{File: file, Line: li + 1, Text: text})
		}
	}
	return out
}

// blankOptArgs blanks any sequence of immediately-following `[...]` optional
// arguments in line, in place on buf. Handles bracket nesting (e.g. inner
// `[]` inside an option list). Stops on the first non-`[` byte. Returns the
// new index after the consumed sequence. Backslash-escaped brackets inside
// the option body are honored (the byte after `\` is skipped without
// triggering depth changes).
func blankOptArgs(line string, buf []byte, i int) int {
	for i < len(line) && line[i] == '[' {
		depth := 1
		buf[i] = ' '
		i++
		for i < len(line) && depth > 0 {
			switch line[i] {
			case '\\':
				buf[i] = ' '
				if i+1 < len(line) {
					buf[i+1] = ' '
					i += 2
					continue
				}
				i++
			case '[':
				depth++
				buf[i] = ' '
				i++
			case ']':
				depth--
				buf[i] = ' '
				i++
			default:
				buf[i] = ' '
				i++
			}
		}
		if depth > 0 {
			// Unclosed bracket — abandon and let outer state machine handle.
			return i
		}
	}
	return i
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
