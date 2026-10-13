// Package texaccent recognises TeX accent / diacritic / special-letter macros
// and resolves them via caller-supplied callbacks. Consumers pick their own
// output flavour (ASCII for bib keys, precomposed Unicode for spell-check).
package texaccent

import "regexp"

const (
	nonAlphaCmd   = `['"` + "`" + `\^~.=]`
	alphaCmd      = `[uvHct]`
	standaloneCmd = `ss|SS|ng|dh|DH|th|TH|ae|AE|oe|OE|aa|AA|[oOlLij]`
)

var (
	// NonAlphaAccent matches all four TeX forms for non-letter accent
	// commands X ∈ {' " ` ^ ~ . =} on a single letter L:
	//
	//   {\X{L}}   {\XL}   \X{L}   \XL
	//
	// Alternation order is longest-first so the outer-brace forms win when
	// they apply (Go's regexp engine is leftmost-first, not leftmost-longest).
	NonAlphaAccent = regexp.MustCompile(
		`\{\\(` + nonAlphaCmd + `)\{([a-zA-Z])\}\}` +
			`|\{\\(` + nonAlphaCmd + `)([a-zA-Z])\}` +
			`|\\(` + nonAlphaCmd + `)\{([a-zA-Z])\}` +
			`|\\(` + nonAlphaCmd + `)([a-zA-Z])`)

	// AlphaAccent matches the braced forms for letter-named accent
	// commands X ∈ {u v H c t} on a single letter L:
	//
	//   {\X{L}}   \X{L}
	//
	// Unbraced forms are NOT matched because they collide with multi-letter
	// command names (`\url`, `\vspace`, `\Huge`, `\centering`, `\textbf`).
	AlphaAccent = regexp.MustCompile(
		`\{\\(` + alphaCmd + `)\{([a-zA-Z])\}\}` +
			`|\\(` + alphaCmd + `)\{([a-zA-Z])\}`)

	// Standalone matches a zero-argument letter command in any of four forms:
	//
	//   {\cmd{}}            (outer-brace, with empty arg suffix)
	//   {\cmd}              (outer-brace)
	//   \cmd{}              (LaTeX idiom: empty-arg terminator, no space inserted)
	//   \cmd<boundary>      (control word followed by space/tab, non-letter, or EOL)
	//
	// The outer-brace and empty-arg forms consume their enclosing tokens, so
	// the boundary reported to the callback is empty. The control-word form
	// reports the boundary byte so the callback can decide whether to keep it
	// (preserve punctuation) or drop it (TeX control-word terminator space).
	Standalone = regexp.MustCompile(
		`\{\\(` + standaloneCmd + `)\{\}\}` +
			`|\{\\(` + standaloneCmd + `)\}` +
			`|\\(` + standaloneCmd + `)\{\}` +
			`|\\(` + standaloneCmd + `)([ \t]|[^a-zA-Z]|$)`)
)

// AccentFn maps an accent command and its base letter to a replacement.
// Returning ok=false leaves the macro untouched.
type AccentFn func(cmd, letter string) (string, bool)

// StandaloneFn maps a zero-arg letter command + its trailing boundary byte
// to a full replacement. The callback decides whether to keep the boundary
// (e.g. preserve a comma) or eat it (e.g. control-word space).
// Returning ok=false leaves the macro untouched.
type StandaloneFn func(cmd, boundary string) (string, bool)

// firstNonEmpty returns the first non-empty string in pairs (cmd, letter).
// Each pair is consecutive capture groups; we step in 2s.
func firstNonEmpty(sub []string) (cmd, letter string) {
	for i := 1; i+1 < len(sub); i += 2 {
		if sub[i] != "" {
			return sub[i], sub[i+1]
		}
	}
	return "", ""
}

// ApplyNonAlphaAccent rewrites every form of `\Xl` for non-letter X.
func ApplyNonAlphaAccent(s string, fn AccentFn) string {
	return NonAlphaAccent.ReplaceAllStringFunc(s, func(m string) string {
		cmd, letter := firstNonEmpty(NonAlphaAccent.FindStringSubmatch(m))
		if repl, ok := fn(cmd, letter); ok {
			return repl
		}
		return m
	})
}

// ApplyAlphaAccent rewrites `\X{l}` (and its outer-brace form) for X ∈ {u v H c t}.
func ApplyAlphaAccent(s string, fn AccentFn) string {
	return AlphaAccent.ReplaceAllStringFunc(s, func(m string) string {
		cmd, letter := firstNonEmpty(AlphaAccent.FindStringSubmatch(m))
		if repl, ok := fn(cmd, letter); ok {
			return repl
		}
		return m
	})
}

// ApplyStandalone rewrites `\cmd<boundary>` and `{\cmd}` / `{\cmd{}}` for
// zero-arg letter commands. The callback receives cmd plus the trailing
// boundary byte (empty for outer-brace forms) and decides whether to keep it.
func ApplyStandalone(s string, fn StandaloneFn) string {
	return Standalone.ReplaceAllStringFunc(s, func(m string) string {
		sub := Standalone.FindStringSubmatch(m)
		var cmd, boundary string
		switch {
		case sub[1] != "":
			cmd = sub[1]
		case sub[2] != "":
			cmd = sub[2]
		case sub[3] != "":
			cmd = sub[3]
		default:
			cmd = sub[4]
			boundary = sub[5]
		}
		if repl, ok := fn(cmd, boundary); ok {
			return repl
		}
		return m
	})
}

// Expand applies all three rewrites in the safe order:
// non-alpha accents → standalones → alpha accents.
func Expand(s string, accent AccentFn, standalone StandaloneFn) string {
	s = ApplyNonAlphaAccent(s, accent)
	s = ApplyStandalone(s, standalone)
	s = ApplyAlphaAccent(s, accent)
	return s
}
