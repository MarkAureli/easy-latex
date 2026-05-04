package spell

import (
	"regexp"
	"strings"
)

// TeX `\ss` / `\SS` produce the German sharp-s glyph (ß / ẞ). For spell-check
// we collapse all of {ß, ẞ, \ss{}, {\ss}, {\ss{}}, \ss<bound>} to plain
// `ss`/`SS` so a single dict entry (e.g. `Bussmann`) covers every source
// spelling. This is NOT length-preserving; spell is the only ProseRuns
// consumer, so column reports degrade slightly on lines containing these
// forms but tokens stay glued together for hunspell.

var (
	ssGroupRe   = regexp.MustCompile(`\{\\ss(?:\{\})?\}`)
	ssBracesRe  = regexp.MustCompile(`\\ss\{\}`)
	ssSpaceLetRe = regexp.MustCompile(`\\ss[ \t]+([A-Za-z])`)
	ssBoundRe   = regexp.MustCompile(`\\ss([^A-Za-z@])`)
	ssEndRe     = regexp.MustCompile(`\\ss\z`)

	bigSSGroupRe    = regexp.MustCompile(`\{\\SS(?:\{\})?\}`)
	bigSSBracesRe   = regexp.MustCompile(`\\SS\{\}`)
	bigSSSpaceLetRe = regexp.MustCompile(`\\SS[ \t]+([A-Za-z])`)
	bigSSBoundRe    = regexp.MustCompile(`\\SS([^A-Za-z@])`)
	bigSSEndRe      = regexp.MustCompile(`\\SS\z`)
)

// umlautRe matches every TeX umlaut spelling for a single ASCII vowel, in
// longest-form-first order so leftmost-first alternation picks the maximal
// span: `{\"{u}}`, `{\"u}`, `\"{u}`, `\"u`. The captured letter (one of four
// alternation branches) is the inside vowel.
var umlautRe = regexp.MustCompile(`\{\\"\{([aeiouyAEIOUY])\}\}|\{\\"([aeiouyAEIOUY])\}|\\"\{([aeiouyAEIOUY])\}|\\"([aeiouyAEIOUY])`)

var umlautMap = map[byte]string{
	'a': "ä", 'e': "ë", 'i': "ï", 'o': "ö", 'u': "ü", 'y': "ÿ",
	'A': "Ä", 'E': "Ë", 'I': "Ï", 'O': "Ö", 'U': "Ü", 'Y': "Ÿ",
}

// NormalizeUmlauts rewrites every TeX umlaut macro form to the precomposed
// UTF-8 character so a German dict entry like `für` covers `f\"ur`,
// `f\"{u}r`, `f{\"u}r`, `f{\"{u}}r`, and the literal `für`.
func NormalizeUmlauts(s string) string {
	return umlautRe.ReplaceAllStringFunc(s, func(m string) string {
		for i := 0; i < len(m); i++ {
			c := m[i]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				if r, ok := umlautMap[c]; ok {
					return r
				}
			}
		}
		return m
	})
}

// NormalizeSharpS rewrites every German sharp-s spelling to plain `ss`/`SS`.
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
