package pedantic

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func init() {
	Register(Check{
		Name:    "braced-args",
		Phase:   PhaseSource,
		Source:  checkBracedArgs,
		Fix:     fixBracedArgs,
		WantRaw: true,
	})
}

// mathFieldPrims are TeX primitives taking one math field; they are not
// macros, so neither the probe nor usage knows them.
var mathFieldPrims = map[string]int{
	"mathop": 1, "mathbin": 1, "mathrel": 1, "mathord": 1, "mathopen": 1,
	"mathclose": 1, "mathpunct": 1, "mathinner": 1, "overline": 1, "underline": 1,
}

// verbatimArgCmds read their argument with changed catcodes (\url|x|), so a
// following character is not a TeX argument token.
var verbatimArgCmds = map[string]bool{
	"url": true, "path": true, "nolinkurl": true, "verb": true, "Verb": true,
	"lstinline": true, "mintinline": true, "href": true,
}

// notUnits are control sequences that cannot stand alone as an argument
// (delimiter-taking or atom-modifying): bracing them would break the source.
var notUnits = map[string]bool{
	"left": true, "right": true, "middle": true, "not": true,
	"big": true, "Big": true, "bigg": true, "Bigg": true,
	"bigl": true, "Bigl": true, "biggl": true, "Biggl": true,
	"bigr": true, "Bigr": true, "biggr": true, "Biggr": true,
	"bigm": true, "Bigm": true, "biggm": true, "Biggm": true,
}

// textAccentSymbols are the control-symbol text accents (\"o, \'e, …), each
// taking one argument. In tabbing, \' \` and \= are tab commands instead.
var textAccentSymbols = map[byte]bool{'\'': true, '`': true, '^': true, '"': true, '~': true, '=': true, '.': true}

const tabbingSymbols = "'`="

// textAccentWords are letter-named text accents (\c c, \v s); like the symbol
// accents they are only braced in prose, where they cannot be a math macro of
// the same name.
var textAccentWords = map[string]bool{"c": true, "v": true, "H": true, "u": true, "b": true, "d": true, "t": true, "r": true, "k": true}

// zeroArgUnits are argument-free text letters that commonly form an
// accent's argument (\'\i) even when the project table does not list them.
var zeroArgUnits = map[string]bool{
	"i": true, "j": true, "ae": true, "AE": true, "oe": true, "OE": true,
	"aa": true, "AA": true, "o": true, "O": true, "l": true, "L": true, "ss": true,
}

// argKind selects how arguments are read.
type argKind uint8

const (
	kindScript argKind = iota // ^ and _: one math field, any single token
	kindMacro                 // macro: optional [...] skipped, no punctuation
	kindAccent                // text accent: no options, no punctuation
)

// braceEdit replaces line[start:end] (optional whitespace plus a bare
// argument token) by repl ("{token}").
type braceEdit struct {
	start, end int
	repl       string
	what       string // diagnostic subject
}

// braceScanner finds bare arguments in one comment-stripped line.
type braceScanner struct {
	ctx     *Context
	line    string
	cls     []texscan.Class
	math    []bool // byte lies in math (texscan.ClassifyMath)
	tabbing bool   // inside a tabbing environment
}

// accentSymbol reports whether the control symbol at i (a backslash) is a text
// accent here.
func (s *braceScanner) accentSymbol(i int) bool {
	if i+1 >= len(s.line) || !textAccentSymbols[s.line[i+1]] {
		return false
	}
	return !s.tabbing || strings.IndexByte(tabbingSymbols, s.line[i+1]) < 0
}

// arity returns the mandatory-argument count of command name (0 if unknown).
func (s *braceScanner) arity(name string) (int, bool) {
	if n, ok := s.ctx.Args(name); ok {
		return n, true
	}
	n, ok := mathFieldPrims[name]
	return n, ok
}

// csName returns the end of the control word starting at the backslash at i,
// including a star when the starred name is a known command, or -1.
func (s *braceScanner) csName(i int) (name string, end int) {
	j := i + 1
	for j < len(s.line) && isASCIILetter(s.line[j]) {
		j++
	}
	if j == i+1 {
		return "", -1
	}
	name = s.line[i+1 : j]
	if j < len(s.line) && s.line[j] == '*' {
		if _, ok := s.arity(name + "*"); !ok {
			return name + "*", -1 // unknown starred form: leave alone
		}
		return name + "*", j + 1
	}
	return name, j
}

func (s *braceScanner) skipBlanks(p int) int {
	for p < len(s.line) && (s.line[p] == ' ' || s.line[p] == '\t') {
		p++
	}
	return p
}

// group returns the end of the balanced open…close group at p (escape-aware),
// or -1 when it does not close on this line.
func (s *braceScanner) group(p int, open, close byte) int {
	depth := 0
	for i := p; i < len(s.line); i++ {
		switch s.line[i] {
		case '\\':
			i++
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// args scans n mandatory arguments starting at pos (right after a command or
// a script character). Braced arguments are kept; bare ones become edits.
// Optional [...] arguments are skipped for macros; accents only take prose
// arguments. ok is false when an
// argument is missing on this line or cannot be braced safely; the caller
// then leaves the whole construct alone.
func (s *braceScanner) args(pos, n int, kind argKind) (edits []braceEdit, end int, ok bool) {
	for k := 0; k < n; {
		p := s.skipBlanks(pos)
		if p >= len(s.line) {
			return nil, 0, false
		}
		if c := s.cls[p]; c == texscan.ClassOpaque || c == texscan.ClassVerbatim || kind == kindAccent && s.math[p] && !c.IsProse() {
			return nil, 0, false // accents: text only (\text{\"o} in math is text)
		}
		switch s.line[p] {
		case '[':
			if kind != kindMacro {
				break
			}
			e := s.group(p, '[', ']')
			if e < 0 {
				return nil, 0, false
			}
			pos = e
			continue
		case '{':
			e := s.group(p, '{', '}')
			if e < 0 {
				return nil, 0, false
			}
			pos = e
			k++
			continue
		}
		unit, e, ok := s.unit(p, kind)
		if !ok {
			return nil, 0, false
		}
		if endsInControlWord(s.line[p:e]) {
			// TeX swallowed the blanks after \i in \'\i ez; once braced
			// (\'{\i}ez) they would become a visible space.
			e = s.skipBlanks(e)
		}
		edits = append(edits, braceEdit{start: pos, end: e, repl: "{" + unit + "}"})
		pos = e
		k++
	}
	return edits, pos, true
}

// unit returns the normalised text and end of the single-token argument at
// p: a control sequence together with its own (recursively braced)
// arguments, a macro parameter, one ASCII letter or digit, one UTF-8
// character, or (for scripts only) one other ASCII character. A text accent
// symbol counts as a control sequence with one argument (\textbf\"o).
func (s *braceScanner) unit(p int, kind argKind) (string, int, bool) {
	c := s.line[p]
	switch {
	case c == '\\':
		if p+1 >= len(s.line) {
			return "", 0, false
		}
		if !isASCIILetter(s.line[p+1]) { // control symbol
			if strings.IndexByte("\\ ", s.line[p+1]) >= 0 {
				return "", 0, false
			}
			if s.accentSymbol(p) {
				return s.withArgs(p, p+2, 1, kindAccent)
			}
			return s.line[p : p+2], p + 2, true
		}
		name, end := s.csName(p)
		if end < 0 || notUnits[name] || verbatimArgCmds[name] {
			return "", 0, false
		}
		n, known := s.arity(name)
		if !known && !zeroArgUnits[name] {
			return "", 0, false // might consume more tokens
		}
		if n == 0 {
			return s.line[p:end], end, true
		}
		k := kindMacro
		if textAccentWords[name] {
			k = kindAccent
		}
		return s.withArgs(p, end, n, k)
	case c == '#':
		if p+1 < len(s.line) && s.line[p+1] >= '1' && s.line[p+1] <= '9' {
			return s.line[p : p+2], p + 2, true
		}
		return "", 0, false
	case isASCIILetter(c) || c >= '0' && c <= '9':
		return s.line[p : p+1], p + 1, true
	case c >= utf8.RuneSelf:
		_, size := utf8.DecodeRuneInString(s.line[p:])
		return s.line[p : p+size], p + size, true
	case kind != kindScript || strings.IndexByte("{}$&%^_~", c) >= 0 || c < ' ':
		return "", 0, false
	}
	return s.line[p : p+1], p + 1, true
}

// endsInControlWord reports whether unit ends with a control word (\alpha),
// after which TeX ignores blanks.
func endsInControlWord(unit string) bool {
	j := len(unit)
	for j > 0 && isASCIILetter(unit[j-1]) {
		j--
	}
	return j < len(unit) && j > 0 && unit[j-1] == '\\' && (j < 2 || unit[j-2] != '\\')
}

// withArgs returns the unit consisting of the command at p..end plus its n
// arguments, bare ones braced.
func (s *braceScanner) withArgs(p, end, n int, kind argKind) (string, int, bool) {
	edits, e, ok := s.args(end, n, kind)
	if !ok {
		return "", 0, false
	}
	return applyBraceEdits(s.line[p:e], edits, p), e, true
}

// edits returns the non-overlapping brace edits of the line, in order.
func (s *braceScanner) edits() []braceEdit {
	var out []braceEdit
	add := func(es []braceEdit, what string) {
		for _, e := range es {
			e.what = what
			out = append(out, e)
		}
	}
	for i := 0; i < len(s.line); i++ {
		if end := editEnd(out, i); end > 0 {
			i = end - 1 // inside an edit: its nested arguments are rebuilt there
			continue
		}
		switch c := s.line[i]; c {
		case '\\':
			if i+1 < len(s.line) && !isASCIILetter(s.line[i+1]) {
				if s.cls[i] == texscan.ClassMarkup && s.accentSymbol(i) {
					if es, _, ok := s.args(i+2, 1, kindAccent); ok {
						add(es, `argument of \`+s.line[i+1:i+2])
					}
				}
				i++ // control symbol (\$, \^, \\, …)
				continue
			}
			name, end := s.csName(i)
			if end < 0 {
				i += len(name)
				continue
			}
			if s.cls[i] == texscan.ClassMarkup && !verbatimArgCmds[name] {
				if n, _ := s.arity(name); n > 0 {
					kind := kindMacro
					if textAccentWords[name] {
						kind = kindAccent
					}
					if es, _, ok := s.args(end, n, kind); ok {
						add(es, `argument of \`+name)
					}
				}
			}
			i = end - 1
		case '^', '_':
			if i+1 < len(s.line) && s.line[i+1] == c {
				i++ // ^^ character notation, not a script
				continue
			}
			if s.cls[i] != texscan.ClassMath {
				continue
			}
			what := "superscript"
			if c == '_' {
				what = "subscript"
			}
			if es, _, ok := s.args(i+1, 1, kindScript); ok {
				add(es, what)
			}
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].start < out[b].start })
	return out
}

// editEnd returns the end of the edit covering i, or 0.
func editEnd(edits []braceEdit, i int) int {
	for _, e := range edits {
		if i >= e.start && i < e.end {
			return e.end
		}
	}
	return 0
}

// applyBraceEdits applies edits (positions relative to the full line; text
// starts at offset off) from right to left.
func applyBraceEdits(text string, edits []braceEdit, off int) string {
	for k := len(edits) - 1; k >= 0; k-- {
		e := edits[k]
		text = text[:e.start-off] + e.repl + text[e.end-off:]
	}
	return text
}

func bracedArgEdits(ctx *Context, stripped []string) [][]braceEdit {
	classes, maths := texscan.ClassifyMath(stripped)
	out := make([][]braceEdit, len(stripped))
	tabbing := false
	for i, line := range stripped {
		// A line opening or closing tabbing counts as inside (conservative).
		opens := strings.Contains(line, `\begin{tabbing}`)
		out[i] = (&braceScanner{ctx: ctx, line: line, cls: classes[i], math: maths[i], tabbing: tabbing || opens}).edits()
		if opens {
			tabbing = true
		}
		if strings.Contains(line, `\end{tabbing}`) {
			tabbing = false
		}
	}
	return out
}

// checkBracedArgs flags single-token arguments written without braces:
// sub- and superscripts in math (x^2) and mandatory arguments of commands
// whose argument count ctx knows (\frac12, \mathbb R, \textbf x).
func checkBracedArgs(ctx *Context, path string, lines []string) []Diagnostic {
	var diags []Diagnostic
	for i, edits := range bracedArgEdits(ctx, stripComments(lines)) {
		for _, e := range edits {
			col := argColumn(lines[i], e.start)
			diags = append(diags, Diagnostic{
				File:    path,
				Line:    i + 1,
				Message: fmt.Sprintf("unbraced %s at column %d", e.what, col),
			})
		}
	}
	return diags
}

// argColumn returns the 1-based column of the first non-blank byte at or
// after p.
func argColumn(line string, p int) int {
	for p < len(line) && (line[p] == ' ' || line[p] == '\t') {
		p++
	}
	return p + 1
}

// fixBracedArgs braces every bare argument, repeating until nothing changes
// (bracing x^\frac{\mathbb R}2 exposes \mathbb R only in the next pass).
func fixBracedArgs(ctx *Context, path string, lines []string) ([]string, bool) {
	changed := false
	for pass := 0; pass < 8; pass++ {
		stripped := stripComments(lines)
		again := false
		for i, edits := range bracedArgEdits(ctx, stripped) {
			if len(edits) == 0 {
				continue
			}
			comment := lines[i][len(stripped[i]):]
			lines[i] = applyBraceEdits(stripped[i], edits, 0) + comment
			again = true
		}
		if !again {
			break
		}
		changed = true
	}
	return lines, changed
}
