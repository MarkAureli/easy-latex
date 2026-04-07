package bib

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	reMath = regexp.MustCompile(`\$\$[^$]*\$\$|\$[^$]*\$`)

	// Non-alphabetic accent commands ('  " ` ^ ~ . =) are always single-char
	// command names and are safe to match in both braced and unbraced form.
	reNonAlphaAccent = regexp.MustCompile(`\\(['"` + "`" + `\^~.=])(?:\{([a-zA-Z])\}|([a-zA-Z]))`)

	// Alphabetic accent commands (u v H c t) share their single letter with
	// the start of longer command names (\url, \vspace, \Huge, \centering,
	// \textbf). Only match the braced form to avoid false positives.
	reAlphaAccent = regexp.MustCompile(`\\([uvHct])\{([a-zA-Z])\}`)

	reStandalone = regexp.MustCompile(`\\(ss|SS|ng|dh|DH|th|TH|ae|AE|oe|OE|aa|AA|[oOlLij])([^a-zA-Z]|$)`)
	reCmdArg     = regexp.MustCompile(`\\[a-zA-Z]+\*?\{([^{}]*)\}`)
	reCmdNoArg   = regexp.MustCompile(`\\[a-zA-Z]+\*?`)
	reBraces     = regexp.MustCompile(`[{}]`)
	reSplit      = regexp.MustCompile(`[^a-zA-Z0-9]+`)
)

// standaloneMap maps zero-argument LaTeX letter commands to their ASCII equivalents.
var standaloneMap = map[string]string{
	"ss": "ss", "SS": "SS",
	"ae": "ae", "AE": "Ae",
	"oe": "oe", "OE": "Oe",
	"aa": "a", "AA": "A",
	"o": "o", "O": "O",
	"l": "l", "L": "L",
	"i": "i", "j": "j",
	"ng": "ng",
	"dh": "d", "DH": "D",
	"th": "th", "TH": "Th",
}

// unicodeMap maps accented Unicode characters to their ASCII equivalents.
// Umlauts expand to two-letter digraphs; all others collapse to the base letter.
var unicodeMap = map[rune]string{
	// a variants
	'á': "a", 'à': "a", 'â': "a", 'ã': "a", 'å': "a", 'ā': "a", 'ă': "a", 'ą': "a",
	'Á': "A", 'À': "A", 'Â': "A", 'Ã': "A", 'Å': "A", 'Ā': "A", 'Ă': "A", 'Ą': "A",
	'ä': "ae", 'Ä': "Ae", 'æ': "ae", 'Æ': "Ae",
	// e variants
	'é': "e", 'è': "e", 'ê': "e", 'ě': "e", 'ë': "e", 'ē': "e", 'ĕ': "e", 'ę': "e",
	'É': "E", 'È': "E", 'Ê': "E", 'Ě': "E", 'Ë': "E", 'Ē': "E", 'Ĕ': "E", 'Ę': "E",
	// i variants
	'í': "i", 'ì': "i", 'î': "i", 'ï': "i", 'ī': "i", 'ĭ': "i", 'į': "i",
	'Í': "I", 'Ì': "I", 'Î': "I", 'Ï': "I", 'Ī': "I", 'Ĭ': "I", 'Į': "I",
	// o variants
	'ó': "o", 'ò': "o", 'ô': "o", 'õ': "o", 'ō': "o", 'ŏ': "o", 'ő': "o",
	'Ó': "O", 'Ò': "O", 'Ô': "O", 'Õ': "O", 'Ō': "O", 'Ŏ': "O", 'Ő': "O",
	'ö': "oe", 'Ö': "Oe", 'ø': "oe", 'Ø': "Oe",
	// u variants
	'ú': "u", 'ù': "u", 'û': "u", 'ū': "u", 'ŭ': "u", 'ů': "u", 'ű': "u", 'ų': "u",
	'Ú': "U", 'Ù': "U", 'Û': "U", 'Ū': "U", 'Ŭ': "U", 'Ů': "U", 'Ű': "U", 'Ų': "U",
	'ü': "ue", 'Ü': "Ue",
	// other
	'ý': "y", 'ÿ': "y", 'Ý': "Y",
	'ß': "ss",
	'ç': "c", 'Ç': "C",
	'ñ': "n", 'Ñ': "N",
	'č': "c", 'Č': "C", 'ć': "c", 'Ć': "C",
	'š': "s", 'Š': "S", 'ś': "s", 'Ś': "S",
	'ž': "z", 'Ž': "Z", 'ź': "z", 'Ź': "Z", 'ż': "z", 'Ż': "Z",
	'ř': "r", 'Ř': "R",
	'ď': "d", 'Ď': "D",
	'ť': "t", 'Ť': "T",
	'ň': "n", 'Ň': "N",
	'ĺ': "l", 'ľ': "l", 'ļ': "l", 'Ĺ': "L", 'Ľ': "L", 'Ļ': "L",
}

// resolveAccent converts a LaTeX accent command and base letter to ASCII.
// Umlauts (cmd == `"`) on a, o, u expand to ae, oe, ue; all others strip the accent.
func resolveAccent(cmd, letter string) string {
	isUpper := letter != strings.ToLower(letter)
	lower := strings.ToLower(letter)

	var base string
	if cmd == `"` {
		switch lower {
		case "a":
			base = "ae"
		case "o":
			base = "oe"
		case "u":
			base = "ue"
		default:
			base = lower
		}
	} else {
		base = lower
	}

	if isUpper && len(base) > 0 {
		return strings.ToUpper(base[:1]) + base[1:]
	}
	return base
}

// accentFunc returns a ReplaceAllStringFunc handler for the given compiled
// accent regex. group2 and group3 are the capture group indices for the letter
// (braced and unbraced respectively; pass -1 if the regex has no unbraced group).
func accentFunc(re *regexp.Regexp, bracedGroup, unbracedGroup int) func(string) string {
	return func(m string) string {
		sub := re.FindStringSubmatch(m)
		letter := sub[bracedGroup]
		if unbracedGroup > 0 && letter == "" {
			letter = sub[unbracedGroup]
		}
		return resolveAccent(sub[1], letter)
	}
}

// latexToASCII converts a LaTeX string to plain ASCII for key generation.
//
// Processing order:
//  1. Strip math mode ($…$ and $$…$$).
//  2. Resolve non-alphabetic accent commands (\'e, \"{u}, etc.) — safe
//     in both braced and unbraced form because their command character
//     (', ", `, ^, ~, ., =) cannot start a multi-letter command name.
//  3. Resolve zero-argument letter commands (\ss, \ae, etc.) before the
//     reCmdArg loop, which would otherwise consume \ss{} as a generic
//     command with empty content.
//  4. Strip \cmd{content} → content (loop for nesting); handles \textbf,
//     \url, \emph and other alphabetic-named commands.
//  5. Resolve alphabetic accent commands in braced form only (\u{a}, \c{c}).
//  6. Strip remaining \cmd.
//  7. Strip remaining braces.
//  8. Map Unicode accented characters to ASCII.
func latexToASCII(s string) string {
	// 1. Strip math mode.
	s = reMath.ReplaceAllString(s, " ")

	// 2. Non-alphabetic accent commands (braced or unbraced).
	s = reNonAlphaAccent.ReplaceAllStringFunc(s, accentFunc(reNonAlphaAccent, 2, 3))

	// 3. Zero-argument letter commands (\ss → ss, \ae → ae, etc.).
	// Must run before reCmdArg so that \ss{} is not swallowed as a generic
	// command with empty content. The terminating non-alpha character is re-appended.
	s = reStandalone.ReplaceAllStringFunc(s, func(m string) string {
		sub := reStandalone.FindStringSubmatch(m)
		if repl, ok := standaloneMap[sub[1]]; ok {
			return repl + sub[2]
		}
		return m
	})

	// 4. Strip \cmd{content} → content (loop handles nested braces inside-out).
	for {
		next := reCmdArg.ReplaceAllString(s, "$1")
		if next == s {
			break
		}
		s = next
	}

	// 5. Alphabetic accent commands, braced form only.
	s = reAlphaAccent.ReplaceAllStringFunc(s, accentFunc(reAlphaAccent, 2, -1))

	// 6. Strip remaining \cmd.
	s = reCmdNoArg.ReplaceAllString(s, " ")

	// 7. Strip remaining braces.
	s = reBraces.ReplaceAllString(s, "")

	// 8. Map Unicode accented characters; drop anything else outside ASCII.
	var sb strings.Builder
	for _, r := range s {
		if repl, ok := unicodeMap[r]; ok {
			sb.WriteString(repl)
		} else if r < 128 {
			sb.WriteRune(r)
		} else {
			sb.WriteRune(' ')
		}
	}
	return sb.String()
}

// toCamelCase splits s on runs of non-alphanumeric characters and returns
// the parts joined with each part's first letter uppercased and rest lowercased.
func toCamelCase(s string) string {
	var sb strings.Builder
	for _, word := range reSplit.Split(s, -1) {
		if word == "" {
			continue
		}
		runes := []rune(word)
		sb.WriteRune(unicode.ToUpper(runes[0]))
		for _, r := range runes[1:] {
			sb.WriteRune(unicode.ToLower(r))
		}
	}
	return sb.String()
}

// firstAuthorLastName returns the last name of the first author from a BibTeX
// author string. Handles both "Last, First" and "First Last" formats.
func firstAuthorLastName(s string) string {
	// Drop everything from " and " onward (additional authors).
	if i := strings.Index(strings.ToLower(s), " and "); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)

	// "Last, First" — take everything before the comma.
	if i := strings.Index(s, ","); i >= 0 {
		return strings.TrimSpace(s[:i])
	}

	// "First Last" — take the last whitespace-separated token.
	if parts := strings.Fields(s); len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return s
}

// GenerateKey produces a canonical citation key for e:
//
//	{FirstAuthorLastNameCamelCase}{Year}{TitleCamelCase}
//
// LaTeX accents are resolved to ASCII; math mode and other special characters
// are stripped. Falls back to the entry's existing key if any component is empty.
func GenerateKey(e Entry) string {
	lastName := toCamelCase(latexToASCII(firstAuthorLastName(FieldValue(e, "author"))))
	year := reSplit.ReplaceAllString(FieldValue(e, "year"), "")
	title := toCamelCase(latexToASCII(FieldValue(e, "title")))

	if lastName == "" || year == "" || title == "" {
		return e.Key
	}
	return lastName + year + title
}

// assignCanonicalKeys rewrites the Key field of every entry in items to its
// canonical form. Entries that produce the same canonical key are disambiguated
// with a lowercase letter suffix (a, b, c, …).
func assignCanonicalKeys(items []Item) {
	type slot struct {
		itemIdx   int
		canonical string
	}
	var slots []slot
	freq := make(map[string]int)
	for i, item := range items {
		if !item.IsEntry {
			continue
		}
		k := GenerateKey(item.Entry)
		slots = append(slots, slot{i, k})
		freq[k]++
	}
	counter := make(map[string]int)
	for _, s := range slots {
		key := s.canonical
		if freq[key] > 1 {
			key += string(rune('a' + counter[s.canonical]))
			counter[s.canonical]++
		}
		items[s.itemIdx].Entry.Key = key
	}
}
