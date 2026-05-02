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
// comment tails verbatim while still computing region masks from the
// comment-stripped body.
func checkDashes(path string, lines []string) []Diagnostic {
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = texscan.StripComment(l)
	}
	mask := regionMask(stripped)
	var diags []Diagnostic
	for i, body := range stripped {
		fixed := applyDashRules(body, mask[i])
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
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = texscan.StripComment(l)
	}
	mask := regionMask(stripped)
	changed := false
	for i, raw := range lines {
		body := stripped[i]
		comment := raw[len(body):]
		fixed := applyDashRules(body, mask[i])
		if fixed != body {
			lines[i] = fixed + comment
			changed = true
		}
	}
	return lines, changed
}

// applyDashRules processes one line by splitting into region spans and
// applying rules per kind.
func applyDashRules(line string, mask []regionKind) string {
	if len(line) == 0 {
		return line
	}
	var b strings.Builder
	b.Grow(len(line))
	start := 0
	cur := mask[0]
	flush := func(end int) {
		span := line[start:end]
		switch cur {
		case regText:
			b.WriteString(rewriteTextSpan(span))
		case regMath:
			b.WriteString(rewriteMathSpan(span))
		case regVerbatim:
			b.WriteString(span)
		}
	}
	for i := 1; i < len(line); i++ {
		if mask[i] != cur {
			flush(i)
			start = i
			cur = mask[i]
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

// rewriteTextSpan applies rules 1,2,3b,4,5,6,7,8,9 with a fixpoint loop so
// chained dashes (e.g. `1-2-3`, `a -- b -- c`) all collapse.
func rewriteTextSpan(s string) string {
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
