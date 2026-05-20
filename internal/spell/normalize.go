package spell

import (
	"regexp"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texaccent"
)

// TeX accent macros and zero-arg letter commands are rewritten to their
// precomposed Unicode characters so hunspell sees real tokens (`René`,
// `naïve`, `Boğaziçi`) rather than macro debris. Substitutions are NOT
// length-preserving; column offsets in downstream diagnostics may shift on
// lines containing rewrites. Spell is the sole consumer of ProseRuns, so the
// drift is local.

// accentTable maps (cmd, lowercased letter) to the precomposed lowercase form.
// Uppercase variants are derived by case-mapping the result.
var accentTable = map[string]map[byte]rune{
	"'": { // acute
		'a': 'á', 'c': 'ć', 'e': 'é', 'i': 'í', 'l': 'ĺ', 'n': 'ń', 'o': 'ó',
		'r': 'ŕ', 's': 'ś', 'u': 'ú', 'y': 'ý', 'z': 'ź',
	},
	"`": { // grave
		'a': 'à', 'e': 'è', 'i': 'ì', 'o': 'ò', 'u': 'ù',
	},
	"^": { // circumflex
		'a': 'â', 'c': 'ĉ', 'e': 'ê', 'g': 'ĝ', 'h': 'ĥ', 'i': 'î', 'j': 'ĵ',
		'o': 'ô', 's': 'ŝ', 'u': 'û', 'w': 'ŵ', 'y': 'ŷ',
	},
	`"`: { // umlaut / diaeresis
		'a': 'ä', 'e': 'ë', 'i': 'ï', 'o': 'ö', 'u': 'ü', 'y': 'ÿ',
	},
	"~": { // tilde
		'a': 'ã', 'n': 'ñ', 'o': 'õ',
	},
	"=": { // macron
		'a': 'ā', 'e': 'ē', 'i': 'ī', 'o': 'ō', 'u': 'ū', 'y': 'ȳ',
	},
	".": { // dot above
		'c': 'ċ', 'e': 'ė', 'g': 'ġ', 'i': 'i', 'z': 'ż',
	},
	"u": { // breve
		'a': 'ă', 'e': 'ĕ', 'g': 'ğ', 'i': 'ĭ', 'o': 'ŏ', 'u': 'ŭ',
	},
	"v": { // caron
		'c': 'č', 'd': 'ď', 'e': 'ě', 'l': 'ľ', 'n': 'ň', 'r': 'ř',
		's': 'š', 't': 'ť', 'z': 'ž',
	},
	"H": { // double acute
		'o': 'ő', 'u': 'ű',
	},
	"c": { // cedilla
		'c': 'ç', 'g': 'ģ', 'k': 'ķ', 'l': 'ļ', 'n': 'ņ', 'r': 'ŗ', 's': 'ş', 't': 'ţ',
	},
}

// unicodeAccent resolves a TeX accent macro to its precomposed Unicode form.
// On unknown combinations (e.g. `\'q`), returns ok=false so the macro is left
// untouched rather than being silently dropped to the bare letter.
func unicodeAccent(cmd, letter string) (string, bool) {
	if letter == "" {
		return "", false
	}
	lc := strings.ToLower(letter)[0]
	upper := letter[0] != lc
	tbl, ok := accentTable[cmd]
	if !ok {
		return "", false
	}
	r, ok := tbl[lc]
	if !ok {
		return "", false
	}
	if upper {
		return strings.ToUpper(string(r)), true
	}
	return string(r), true
}

// standaloneUnicode maps zero-arg TeX letter commands to precomposed Unicode.
var standaloneUnicode = map[string]string{
	"ss": "ß", "SS": "ẞ",
	"ae": "æ", "AE": "Æ",
	"oe": "œ", "OE": "Œ",
	"aa": "å", "AA": "Å",
	"o": "ø", "O": "Ø",
	"l": "ł", "L": "Ł",
	"i": "ı", "j": "ȷ",
	"ng": "ŋ",
	"dh": "ð", "DH": "Ð",
	"th": "þ", "TH": "Þ",
}

// unicodeStandalone resolves a zero-arg TeX letter command. A single
// space/tab after the macro is eaten (TeX control-word terminator
// semantics) so `\aa ngstrom` glues to `ångstrom`. Other boundary bytes
// (punctuation, newline) are preserved.
func unicodeStandalone(cmd, boundary string) (string, bool) {
	r, ok := standaloneUnicode[cmd]
	if !ok {
		return "", false
	}
	if boundary == " " || boundary == "\t" {
		return r, true
	}
	return r + boundary, true
}

// NormalizeAccents rewrites every TeX accent macro, alphabetic accent macro,
// and zero-arg letter command to its precomposed Unicode character so a
// dictionary entry like `René` covers `Ren\'e`, `Ren\'{e}`, `Ren{\'e}`,
// `Ren{\'{e}}`, and the literal `René`.
func NormalizeAccents(s string) string {
	return texaccent.Expand(s, unicodeAccent, unicodeStandalone)
}

// Legacy German sharp-s collapse. Kept separate from NormalizeAccents because
// it covers raw-glyph forms and TeX command boundaries (e.g. `\ss<space>letter`
// for the `\ss ` followed by `Bann` pattern) that texaccent does not handle.

var (
	ssGroupRe    = regexp.MustCompile(`\{\\ss(?:\{\})?\}`)
	ssBracesRe   = regexp.MustCompile(`\\ss\{\}`)
	ssSpaceLetRe = regexp.MustCompile(`\\ss[ \t]+([A-Za-z])`)
	ssBoundRe    = regexp.MustCompile(`\\ss([^A-Za-z@])`)
	ssEndRe      = regexp.MustCompile(`\\ss\z`)

	bigSSGroupRe    = regexp.MustCompile(`\{\\SS(?:\{\})?\}`)
	bigSSBracesRe   = regexp.MustCompile(`\\SS\{\}`)
	bigSSSpaceLetRe = regexp.MustCompile(`\\SS[ \t]+([A-Za-z])`)
	bigSSBoundRe    = regexp.MustCompile(`\\SS([^A-Za-z@])`)
	bigSSEndRe      = regexp.MustCompile(`\\SS\z`)
)

// NormalizeSharpS rewrites every German sharp-s spelling to plain `ss`/`SS`.
// Applied after NormalizeAccents because hunspell `de_*` dicts commonly key
// the ss-form (`Bussmann`, `Strasse`), not the ß-form.
func NormalizeSharpS(s string) string {
	s = ssGroupRe.ReplaceAllString(s, "ss")
	s = ssBracesRe.ReplaceAllString(s, "ss")
	s = ssSpaceLetRe.ReplaceAllString(s, "ss$1")
	s = ssBoundRe.ReplaceAllString(s, "ss$1")
	s = ssEndRe.ReplaceAllString(s, "ss")

	s = bigSSGroupRe.ReplaceAllString(s, "SS")
	s = bigSSBracesRe.ReplaceAllString(s, "SS")
	s = bigSSSpaceLetRe.ReplaceAllString(s, "SS$1")
	s = bigSSBoundRe.ReplaceAllString(s, "SS$1")
	s = bigSSEndRe.ReplaceAllString(s, "SS")

	s = strings.ReplaceAll(s, "ß", "ss")
	s = strings.ReplaceAll(s, "ẞ", "SS")
	return s
}
