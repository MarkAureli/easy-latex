package pedantic

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func init() {
	Register(Check{
		Name:    "dashes",
		Phase:   PhaseSource,
		Source:  checkDashes,
		Fix:     fixDashes,
		WantRaw: true,
	})
}

const (
	uEnDash = "–" // –
	uEmDash = "—" // —
	uMinus  = "−" // −
)

// checkDashes flags dash-style violations. WantRaw: true so we can preserve
// comment tails verbatim while classifying the comment-stripped body.
func checkDashes(path string, lines []string) []Diagnostic {
	stripped := stripComments(lines)
	classes := texscan.Classify(stripped)
	var diags []Diagnostic
	for i, body := range stripped {
		fixed := applyDashRules(body, classes[i])
		if fixed != body {
			col := firstByteDiff(body, fixed) + 1
			diags = append(diags, Diagnostic{
				File:    path,
				Line:    i + 1,
				Message: fmt.Sprintf("dash style violation at column %d", col),
			})
		}
	}
	return diags
}

// fixDashes rewrites raw lines applying every rule.
func fixDashes(path string, lines []string) ([]string, bool) {
	stripped := stripComments(lines)
	classes := texscan.Classify(stripped)
	changed := false
	for i, raw := range lines {
		body := stripped[i]
		comment := raw[len(body):]
		fixed := applyDashRules(body, classes[i])
		if fixed != body {
			lines[i] = fixed + comment
			changed = true
		}
	}
	return lines, changed
}

// dashSpan groups classes into the spans the dash rules distinguish.
func dashSpan(c texscan.Class) int {
	switch {
	case c.IsProse():
		return 0
	case c == texscan.ClassMath:
		return 1
	default:
		return 2 // markup, opaque, verbatim: untouched
	}
}

// applyDashRules rewrites the prose and math spans of one line; everything
// else (markup, options, arguments of non-prose commands, code, verbatim) is
// left as is.
func applyDashRules(line string, classes []texscan.Class) string {
	if len(line) == 0 {
		return line
	}
	var b strings.Builder
	b.Grow(len(line))
	start := 0
	cur := dashSpan(classes[0])
	flush := func(end int) {
		span := line[start:end]
		switch cur {
		case 0:
			b.WriteString(rewriteTextChunk(span))
		case 1:
			b.WriteString(rewriteMathSpan(span))
		default:
			b.WriteString(span)
		}
	}
	for i := 1; i < len(line); i++ {
		if k := dashSpan(classes[i]); k != cur {
			flush(i)
			start = i
			cur = k
		}
	}
	flush(len(line))
	return b.String()
}

// rewriteMathSpan: rule 3a only.
func rewriteMathSpan(s string) string {
	return strings.ReplaceAll(s, uMinus, "-")
}

var (
	reTriHy    = regexp.MustCompile(`----+`)              // 4+ hyphens
	reDigEm    = regexp.MustCompile(`(\d)\s*---\s*(\d)`)  // digit em → en
	reDigHy    = regexp.MustCompile(`(\d)\s*-\s*(\d)`)    // digit hyphen → en
	reEmSpaces = regexp.MustCompile(` *--- *`)            // strip spaces around em-dash
	reWordEn   = regexp.MustCompile(`(\w+)\s*--\s*(\w+)`) // en-dash between words
	reWordHy   = regexp.MustCompile(`(\w) - (\w)`)        // spaced hyphen between words
)

// rewriteTextChunk applies rules 1,2,3b,4,5,6,7,8,9 to a prose span with a
// fixpoint loop so chained dashes (e.g. `1-2-3`, `a -- b -- c`) all collapse.
func rewriteTextChunk(s string) string {
	for range 8 {
		prev := s
		s = strings.ReplaceAll(s, uEnDash, "--")
		s = strings.ReplaceAll(s, uEmDash, "---")
		s = strings.ReplaceAll(s, uMinus, "$-$")
		s = reTriHy.ReplaceAllString(s, "---")
		s = reDigEm.ReplaceAllString(s, "$1--$2")
		s = reDigHy.ReplaceAllString(s, "$1--$2")
		s = reEmSpaces.ReplaceAllString(s, "---")
		s = reWordEn.ReplaceAllStringFunc(s, replaceWordEn)
		s = reWordHy.ReplaceAllStringFunc(s, replaceWordHy)
		if s == prev {
			break
		}
	}
	return s
}

// replaceWordEn: skip if either side digit OR both sides start uppercase.
func replaceWordEn(m string) string {
	sub := reWordEn.FindStringSubmatch(m)
	a, c := sub[1], sub[2]
	if isDigitStr(a) || isDigitStr(c) {
		return m
	}
	if isUpperStr(a) && isUpperStr(c) {
		return m
	}
	return a + "---" + c
}

// replaceWordHy: skip if either side digit. Cap-cap allowed (per Q2).
func replaceWordHy(m string) string {
	sub := reWordHy.FindStringSubmatch(m)
	a, c := sub[1], sub[2]
	if isDigitStr(a) || isDigitStr(c) {
		return m
	}
	return a + "---" + c
}

func isDigitStr(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsDigit(r)
}

func isUpperStr(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(r)
}

// firstByteDiff returns the index of the first differing byte, or len(min).
func firstByteDiff(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
