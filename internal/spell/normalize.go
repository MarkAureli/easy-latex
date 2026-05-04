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
