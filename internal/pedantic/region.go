package pedantic

import (
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// regionKind classifies a byte position in tex source.
type regionKind uint8

const (
	regText regionKind = iota
	regMath
	regVerbatim
)

// regionMask returns a per-line, per-byte classification of text/math/verbatim,
// derived from texscan.ClassifyMath. Input lines should be comment-stripped
// (texscan.StripComment).
//
// Math covers everything inside math, including command names and arguments
// (\alpha, \text{…}); boundary tokens ($, \[, \begin{equation}, \end{…})
// are regText, so callers that skip non-text regions still observe them (e.g.
// so block-on-newline can flag a mid-line \end{...}). Arguments of commands
// in text are regText.
func regionMask(lines []string) [][]regionKind {
	classes, maths := texscan.ClassifyMath(lines)
	out := make([][]regionKind, len(lines))
	for li, cs := range classes {
		m := make([]regionKind, len(cs))
		for i, c := range cs {
			switch {
			case c == texscan.ClassVerbatim:
				m[i] = regVerbatim
			case maths[li][i]:
				m[i] = regMath
			}
		}
		out[li] = m
	}
	return out
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
