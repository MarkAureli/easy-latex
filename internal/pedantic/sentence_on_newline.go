package pedantic

import (
	"fmt"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func init() {
	Register(Check{
		Name:   "sentence-on-newline",
		Phase:  PhaseSource,
		Source: checkSentenceOnNewline,
		Fix:    fixSentenceOnNewline,
	})
}

// sentenceAbbrevs are tokens that, when they precede a sentence-end period,
// indicate an abbreviation rather than a sentence boundary.
var sentenceAbbrevs = map[string]bool{
	"e.g": true, "i.e": true, "cf": true, "etc": true, "vs": true,
	"Mr": true, "Mrs": true, "Ms": true, "Dr": true, "Prof": true,
	"Fig": true, "Figs": true, "Eq": true, "Eqs": true,
	"No": true, "St": true, "Jr": true, "Sr": true,
	"Sec": true, "Secs": true, "Vol": true, "Ch": true, "Tab": true,
	"approx": true, "resp": true,
}

func checkSentenceOnNewline(path string, lines []string) []Diagnostic {
	mask := regionMask(lines)
	var diags []Diagnostic
	for li, line := range lines {
		for _, pos := range findSentenceSplits(line, mask[li]) {
			diags = append(diags, Diagnostic{
				File:    path,
				Line:    li + 1,
				Message: fmt.Sprintf("sentence should start on a new line (column %d)", pos+1),
			})
		}
	}
	return diags
}

func fixSentenceOnNewline(path string, lines []string) ([]string, bool) {
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = texscan.StripComment(l)
	}
	mask := regionMask(stripped)
	out := make([]string, 0, len(lines))
	changed := false
	for li, line := range lines {
		body := stripped[li]
		comment := line[len(body):]
		splits := findSentenceSplits(body, mask[li])
		if len(splits) == 0 {
			out = append(out, line)
			continue
		}
		out = append(out, splitLineAt(body, comment, body[:leadingWS(body)], splits)...)
		changed = true
	}
	return out, changed
}

// findSentenceSplits returns positions in line at which a new sentence begins
// (the offset of the leading capital letter). Only mid-line boundaries are
// reported; sentences that already end the line are ignored.
func findSentenceSplits(line string, mask []regionKind) []int {
	var out []int
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c != '.' && c != '?' && c != '!' {
			continue
		}
		if i < len(mask) && mask[i] != regText {
			continue
		}
		j := i + 1
		for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
			j++
		}
		if j == i+1 || j >= len(line) {
			continue
		}
		next := line[j]
		if next < 'A' || next > 'Z' {
			continue
		}
		word := precedingWord(line, i)
		if word == "" || allDigits(word) {
			continue
		}
		if sentenceAbbrevs[word] {
			continue
		}
		out = append(out, j)
	}
	return out
}

// precedingWord returns the run of non-whitespace characters immediately
// before position i, with any trailing periods stripped (so "e.g." → "e.g").
func precedingWord(line string, i int) string {
	j := i
	for j > 0 {
		c := line[j-1]
		if c == ' ' || c == '\t' {
			break
		}
		j--
	}
	w := line[j:i]
	for len(w) > 0 && w[len(w)-1] == '.' {
		w = w[:len(w)-1]
	}
	return w
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
