package cmdprobe

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// classifyAll returns the math class of every logged command whose meaning
// shows one. Anything else is absent and falls back to the built-in table
// (texscan.BuiltinMathClasses), then to ordinary.
func classifyAll(meanings map[string]map[string]string) map[string]texscan.MathClass {
	out := map[string]texscan.MathClass{}
	for name, m := range meanings {
		if c, ok := mathClass(name, m, kindBase, 0); ok {
			out[name] = c
		}
	}
	return out
}

// mathClassPrims maps the class-setting primitives a macro body may start
// with (\mathrel{...}) to their class.
var mathClassPrims = map[string]texscan.MathClass{
	`\mathord`: texscan.MathOrd, `\mathop`: texscan.MathOp, `\mathbin`: texscan.MathBin,
	`\mathrel`: texscan.MathRel, `\mathopen`: texscan.MathOpen,
	`\mathclose`: texscan.MathClose, `\mathpunct`: texscan.MathPunct,
}

// mathClass reads the class from the meaning m[kind]: a math character
// (\mathchar"3214, \delimiter"426830A, \Umathchar"3"0"002264, unicode-math's
// "the character ∈"), a parameterless macro whose body is one of those or
// starts with a class primitive (\mathrel{…}), a robust wrapper (\protect
// \foo␣), or an alias to one command whose class the built-in table knows
// (unicode-math's \le → \leq). Anything else is unresolved.
func mathClass(name string, m map[string]string, kind string, depth int) (texscan.MathClass, bool) {
	meaning, ok := m[kind]
	if !ok || depth > 2 {
		return "", false
	}
	params, body, isMacro := splitMacro(meaning)
	if !isMacro {
		return charClass(meaning)
	}
	if params != "" {
		return "", false
	}
	body = strings.TrimSpace(body)
	if kind == kindBase && body == `\protect \`+name {
		return mathClass(name, m, kindSpace, depth+1)
	}
	if c, ok := charClass(body); ok {
		return c, true
	}
	head, rest := body, ""
	if i := strings.IndexAny(body, " {"); i >= 0 {
		head, rest = body[:i], body[i:]
	}
	if c, ok := mathClassPrims[head]; ok {
		return c, true
	}
	if rest == "" && len(head) > 1 && head[0] == '\\' && isLetters(head[1:]) {
		c, ok := texscan.BuiltinMathClasses[head[1:]]
		return c, ok
	}
	return "", false
}

// charClass reads the class of a math character meaning; the class sits in
// the top bits of \mathchar (15-bit) and \delimiter (27-bit) values and is
// the first number of \Umathchar and \Udelimiter.
func charClass(s string) (texscan.MathClass, bool) {
	var v int
	var ok bool
	switch {
	case strings.HasPrefix(s, `\Umathchar`):
		v, ok = texNumber(strings.TrimPrefix(s, `\Umathchar`))
	case strings.HasPrefix(s, `\Udelimiter`):
		v, ok = texNumber(strings.TrimPrefix(s, `\Udelimiter`))
	case strings.HasPrefix(s, `\mathchar`):
		v, ok = texNumber(strings.TrimPrefix(s, `\mathchar`))
		v >>= 12
	case strings.HasPrefix(s, `\delimiter`):
		v, ok = texNumber(strings.TrimPrefix(s, `\delimiter`))
		v >>= 24
	case strings.HasPrefix(s, "the character "):
		r, _ := utf8.DecodeRuneInString(strings.TrimPrefix(s, "the character "))
		c, ok := texscan.UnicodeMathClasses[r]
		return c, ok
	}
	if !ok {
		return "", false
	}
	return texscan.MathClassFromDigit(v)
}

// texNumber parses the TeX number at the start of s: optional blanks, then
// "hex, 'octal or decimal digits.
func texNumber(s string) (int, bool) {
	s = strings.TrimLeft(s, " ")
	base := 10
	switch {
	case strings.HasPrefix(s, `"`):
		base, s = 16, s[1:]
	case strings.HasPrefix(s, "'"):
		base, s = 8, s[1:]
	}
	end := 0
	for end < len(s) && strings.IndexByte("0123456789ABCDEF"[:base], s[end]) >= 0 {
		end++
	}
	if end == 0 {
		return 0, false
	}
	v, err := strconv.ParseInt(s[:end], base, 64)
	return int(v), err == nil
}
