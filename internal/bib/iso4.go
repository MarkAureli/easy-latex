package bib

import (
	"bufio"
	"bytes"
	_ "embed"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

//go:embed ltwa.tsv
var ltwaRaw []byte

type ltwaRecord struct {
	abbrev string
	lang   string // lowercased languages field
}

var (
	ltwaOnce   sync.Once
	ltwaExact  map[string][]ltwaRecord // lowercase word → records
	ltwaPrefix map[string][]ltwaRecord // lowercase prefix (without trailing -) → records
)

func loadLTWA() {
	ltwaOnce.Do(func() {
		ltwaExact = make(map[string][]ltwaRecord)
		ltwaPrefix = make(map[string][]ltwaRecord)

		scanner := bufio.NewScanner(bytes.NewReader(ltwaRaw))
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) < 2 {
				continue
			}
			word := strings.ToLower(strings.TrimSpace(parts[0]))
			abbrev := strings.TrimSpace(parts[1])
			lang := ""
			if len(parts) == 3 {
				lang = strings.ToLower(strings.TrimSpace(parts[2]))
			}
			if word == "" || abbrev == "" || strings.HasPrefix(word, "-") {
				continue
			}
			rec := ltwaRecord{abbrev: abbrev, lang: lang}
			if strings.HasSuffix(word, "-") {
				prefix := word[:len(word)-1]
				ltwaPrefix[prefix] = append(ltwaPrefix[prefix], rec)
			} else {
				ltwaExact[word] = append(ltwaExact[word], rec)
			}
		}
	})
}

// pickAbbrev selects the best abbreviation from a slice of LTWA records.
// Priority: "multiple languages" > records containing "english" > first record.
func pickAbbrev(recs []ltwaRecord) string {
	for _, r := range recs {
		if strings.Contains(r.lang, "multiple") {
			return r.abbrev
		}
	}
	for _, r := range recs {
		if strings.Contains(r.lang, "english") {
			return r.abbrev
		}
	}
	return recs[0].abbrev
}

// lookupLTWA returns the ISO 4 abbreviation for a single lowercase word.
// found is false when no LTWA entry exists.
func lookupLTWA(lower string) (abbrev string, found bool) {
	loadLTWA()

	if recs, ok := ltwaExact[lower]; ok {
		return pickAbbrev(recs), true
	}

	// Longest-prefix match: try decreasing substrings of lower.
	// Start at len(lower) so that a word exactly matching a prefix entry (e.g.
	// "review" matching the "review-" entry) is found on the first iteration.
	for i := len(lower); i >= 1; i-- {
		if recs, ok := ltwaPrefix[lower[:i]]; ok {
			return pickAbbrev(recs), true
		}
	}
	return "", false
}

// stopWords lists articles, prepositions, and conjunctions that ISO 4 requires
// be omitted from abbreviated titles. First-position words are not exempt.
var stopWords = map[string]bool{
	// English
	"a": true, "an": true, "the": true,
	"and": true, "but": true, "or": true, "nor": true,
	"of": true, "in": true, "on": true, "at": true, "to": true,
	"by": true, "for": true, "with": true, "from": true, "into": true,
	// French
	"au": true, "aux": true, "de": true, "des": true, "du": true,
	"en": true, "et": true, "la": true, "le": true, "les": true,
	"ou": true, "par": true, "pour": true, "sous": true, "sur": true,
	"un": true, "une": true,
	// German
	"am": true, "auf": true, "aus": true, "bei": true, "das": true,
	"dem": true, "den": true, "der": true, "die": true,
	"ein": true, "eine": true, "für": true, "im": true, "ins": true,
	"mit": true, "nach": true, "oder": true, "und": true, "vom": true,
	"von": true, "vor": true, "zu": true, "zum": true, "zur": true,
	// Italian
	"al": true, "agli": true, "alla": true, "alle": true, "allo": true,
	"col": true, "degli": true, "dei": true, "del": true,
	"della": true, "delle": true, "dello": true, "di": true,
	"e": true, "ed": true, "il": true, "lo": true,
	"nel": true, "nella": true, "nelle": true, "nello": true,
	"sul": true, "sulla": true, "sulle": true, "sullo": true,
	// Spanish
	"con": true, "entre": true, "para": true, "por": true, "y": true,
	// Latin
	"ab": true, "ac": true, "ad": true, "ex": true, "pro": true, "sub": true,
}

// AbbreviateISO4 returns the ISO 4 abbreviated form of a journal title.
// Single-word titles are returned unchanged.
func AbbreviateISO4(title string) string {
	// Strip trailing parenthetical subtitles, e.g. "Lancet (London)" → "Lancet".
	if idx := strings.LastIndex(title, "("); idx > 0 {
		suffix := strings.TrimSpace(title[idx:])
		if strings.HasSuffix(suffix, ")") {
			title = strings.TrimSpace(title[:idx])
		}
	}

	words := strings.Fields(title)
	if len(words) <= 1 {
		return title
	}

	var parts []string
	for _, word := range words {
		part := abbreviateToken(word)
		if part != "" {
			parts = append(parts, part)
		}
	}

	if len(parts) == 0 {
		return title
	}
	return strings.Join(parts, " ")
}

// abbreviateToken returns the abbreviated form of a single whitespace-delimited
// token. Returns "" when the token should be omitted.
func abbreviateToken(word string) string {
	if strings.Contains(word, "-") {
		return abbreviateHyphenated(word)
	}
	return abbreviateSingle(word)
}

func abbreviateHyphenated(word string) string {
	segments := strings.Split(word, "-")
	var out []string
	for _, seg := range segments {
		a := abbreviateSingle(seg)
		if a != "" {
			out = append(out, a)
		}
	}
	return strings.Join(out, "-")
}

func abbreviateSingle(word string) string {
	if word == "" {
		return ""
	}
	lower := strings.ToLower(word)

	abbrev, found := lookupLTWA(lower)
	if found {
		// Mirror the capitalisation of the original word's first letter.
		firstRune, _ := utf8.DecodeRuneInString(word)
		if unicode.IsUpper(firstRune) {
			abbrevRune, abbrevSize := utf8.DecodeRuneInString(abbrev)
			abbrev = string(unicode.ToUpper(abbrevRune)) + abbrev[abbrevSize:]
		}
		return abbrev
	}

	if stopWords[lower] {
		return ""
	}
	return word
}
