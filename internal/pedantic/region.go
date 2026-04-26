package pedantic

import "strings"

// regionKind classifies a byte position in tex source.
type regionKind uint8

const (
	regText regionKind = iota
	regMath
	regVerbatim
)

// verbatimEnvs are environments whose body is treated as opaque verbatim text.
var verbatimEnvs = map[string]bool{
	"verbatim":   true,
	"Verbatim":   true,
	"BVerbatim":  true,
	"LVerbatim":  true,
	"lstlisting": true,
	"minted":     true,
	"comment":    true,
}

// displayMathEnvs are environments treated as math regions.
var displayMathEnvs = map[string]bool{
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

// regionMask returns a per-line, per-byte classification of text/math/verbatim.
// State carries across lines for envs and display math (\[ ... \]). Inline math
// ($...$, \(...\)) is tracked across lines too, though it usually closes on
// the same line. Input lines should be comment-stripped (texscan.StripComment).
//
// Boundary tokens (the bytes of the opener and closer themselves) are
// classified as regText — only the inner content carries the math/verbatim
// kind. This means callers that skip non-text regions still observe the
// boundary tokens (e.g. so block-on-newline can flag a mid-line \end{...}).
func regionMask(lines []string) [][]regionKind {
	out := make([][]regionKind, len(lines))
	state := regText
	envName := ""
	for li, line := range lines {
		m := make([]regionKind, len(line))
		i := 0
		for i < len(line) {
			if state == regText {
				// Escape sequences that must not be interpreted as openers.
				if line[i] == '\\' && i+1 < len(line) {
					nx := line[i+1]
					if nx == '$' || nx == '%' || nx == '\\' {
						m[i] = regText
						m[i+1] = regText
						i += 2
						continue
					}
				}
				if k, n, name, ok := openerAt(line, i); ok {
					for j := range n {
						m[i+j] = regText
					}
					state = k
					envName = name
					i += n
					continue
				}
				m[i] = regText
				i++
				continue
			}
			// state == regMath or regVerbatim
			if n, ok := closerAt(line, i, envName); ok {
				for j := range n {
					m[i+j] = regText
				}
				i += n
				state = regText
				envName = ""
				continue
			}
			// Skip backslash escapes inside math so \$, \\, \(, \) don't
			// trigger spurious closer logic on the next byte.
			if state == regMath && line[i] == '\\' && i+1 < len(line) {
				m[i] = regMath
				m[i+1] = regMath
				i += 2
				continue
			}
			m[i] = state
			i++
		}
		out[li] = m
	}
	return out
}

// openerAt detects an opener of a math or verbatim region at position i.
// Returns (kind, length, envName, ok). envName uniquely identifies the closer
// to look for: "$", "$$", "\\(", "\\[" for inline forms; the env name for
// \begin{name}.
func openerAt(s string, i int) (regionKind, int, string, bool) {
	if i >= len(s) {
		return 0, 0, "", false
	}
	c := s[i]
	if c == '$' {
		if i+1 < len(s) && s[i+1] == '$' {
			return regMath, 2, "$$", true
		}
		return regMath, 1, "$", true
	}
	if c == '\\' && i+1 < len(s) {
		switch s[i+1] {
		case '(':
			return regMath, 2, "\\(", true
		case '[':
			return regMath, 2, "\\[", true
		}
		if name, n := matchBeginEnd(s, i, "begin"); n > 0 {
			if verbatimEnvs[name] {
				return regVerbatim, n, name, true
			}
			if displayMathEnvs[name] {
				return regMath, n, name, true
			}
		}
	}
	return 0, 0, "", false
}

// closerAt detects a closer at position i for the region identified by envName.
func closerAt(s string, i int, envName string) (int, bool) {
	if i >= len(s) {
		return 0, false
	}
	switch envName {
	case "$":
		if s[i] == '$' {
			return 1, true
		}
	case "$$":
		if i+1 < len(s) && s[i] == '$' && s[i+1] == '$' {
			return 2, true
		}
	case "\\(":
		if i+1 < len(s) && s[i] == '\\' && s[i+1] == ')' {
			return 2, true
		}
	case "\\[":
		if i+1 < len(s) && s[i] == '\\' && s[i+1] == ']' {
			return 2, true
		}
	default:
		if s[i] == '\\' {
			if name, n := matchBeginEnd(s, i, "end"); n > 0 && name == envName {
				return n, true
			}
		}
	}
	return 0, false
}

// matchBeginEnd matches "\begin{X}" or "\end{X}" at i. Returns (name, length)
// or ("", 0) if no match.
func matchBeginEnd(s string, i int, kw string) (string, int) {
	prefix := "\\" + kw + "{"
	if !strings.HasPrefix(s[i:], prefix) {
		return "", 0
	}
	rest := s[i+len(prefix):]
	end := strings.IndexByte(rest, '}')
	if end < 0 {
		return "", 0
	}
	return rest[:end], len(prefix) + end + 1
}

// leadingWS returns the byte offset of the first non-whitespace character on
// the line, or len(line) if the line is empty / all-whitespace.
func leadingWS(line string) int {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i
}

// splitLineAt rebuilds a single source line as multiple sub-lines split at the
// given byte offsets in body. The first sub-line keeps its original lead;
// subsequent sub-lines reapply leadStr. The original comment tail is appended
// to the last sub-line. Trailing whitespace before each split is trimmed.
// splits must be in ascending order and within [0, len(body)].
func splitLineAt(body, comment, leadStr string, splits []int) []string {
	out := make([]string, 0, len(splits)+1)
	prev := 0
	for _, s := range splits {
		chunk := strings.TrimRight(body[prev:s], " \t")
		if prev == 0 {
			out = append(out, chunk)
		} else {
			out = append(out, leadStr+chunk)
		}
		prev = s
	}
	last := body[prev:]
	if prev == 0 {
		out = append(out, last+comment)
	} else {
		out = append(out, leadStr+last+comment)
	}
	return out
}
