package pedantic

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func init() {
	Register(Check{
		Name:    "math-token-spacing",
		Phase:   PhaseSource,
		Source:  checkMathTokenSpacing,
		Fix:     fixMathTokenSpacing,
		WantRaw: true,
	})
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// spaceKind is how a math token behaves towards one neighbour.
type spaceKind uint8

const (
	skAtom       spaceKind = iota // letter, number, ordinary command, script argument
	skBin                         // binary operator (+, \cdot)
	skRel                         // relation (=, \le)
	skPunct                       // , ; . ! \colon: tight before, space after
	skOpen                        // opening delimiter: ( [ \{ \langle \left(
	skClose                       // closing delimiter
	skGroupOpen                   // { or an optional-argument [
	skGroupClose                  // } or an optional-argument ]
	skScript                      // ^ _
	skPrime                       // '
	skAttach                      // \limits \nolimits: tight before
	skLeave                       // | / ~ & \, \quad, non-math content: never touched
	skBoundary                    // $ \( \[ \\ \begin \end: never touched
)

// mathToken is one token of a math line. l and r give its behaviour towards
// its left and right neighbour (they differ for } and primes, which close or
// extend an atom).
type mathToken struct {
	start, end int
	l, r       spaceKind
	fn         bool // r == skAtom and its base is a letter or ordinary command
	unary      bool // sign directly applied to the next token (= -x)
	relChar    bool // relation character (= < > :), kept tight in := and =:
	word       bool // run of 2+ letters
}

// spaceOwner describes what a {…} or […] group belongs to: its closing
// token takes the owner's right-hand behaviour.
type spaceOwner struct {
	r  spaceKind
	fn bool
}

// spacingCmds are spacing commands: their neighbours are never touched.
var spacingCmds = map[string]bool{
	"quad": true, "qquad": true, "enspace": true, "thinspace": true, "medspace": true,
	"thickspace": true, "negthinspace": true, "negmedspace": true, "negthickspace": true,
	"hspace": true, "hspace*": true, "hskip": true, "mskip": true, "kern": true, "mkern": true,
	"hfill": true, "hfil": true, "nobreak": true, "allowbreak": true, "vert": true, "Vert": true,
}

// delimPrefixes take the following delimiter into one token of the given kind
// (skLeave: the delimiter's own kind).
var delimPrefixes = map[string]spaceKind{
	"left": skOpen, "right": skClose, "middle": skRel, "not": skRel,
	"big": skLeave, "Big": skLeave, "bigg": skLeave, "Bigg": skLeave,
	"bigl": skOpen, "Bigl": skOpen, "biggl": skOpen, "Biggl": skOpen,
	"bigr": skClose, "Bigr": skClose, "biggr": skClose, "Biggr": skClose,
	"bigm": skRel, "Bigm": skRel, "biggm": skRel, "Biggm": skRel,
}

var classKinds = map[texscan.MathClass]spaceKind{
	texscan.MathBin: skBin, texscan.MathRel: skRel, texscan.MathOpen: skOpen,
	texscan.MathClose: skClose, texscan.MathPunct: skPunct,
}

// spacingScanner tokenizes the math of a file line by line, carrying group
// and sign state across lines.
type spacingScanner struct {
	ctx     *Context
	stack   []spaceOwner
	prev    spaceKind   // right kind of the last significant token
	atomFn  bool        // fn of the last atom (base of primes and scripts)
	script  bool        // last token was ^ or _: the next one is its argument
	owner   *spaceOwner // command the next {…} / […] group is an argument of
	line    string
	cls     []texscan.Class
	math    []bool
	toks    []mathToken
	pending int // index in toks of the token awaiting its delimiter, or -1
}

func (s *spacingScanner) reset() {
	s.stack, s.prev, s.script, s.owner = nil, skBoundary, false, nil
}

// emit appends a token and updates the cross-token state.
func (s *spacingScanner) emit(t mathToken) {
	if s.script && t.l != skGroupOpen {
		// Bare script argument: an atom continuing the scripted base.
		s.script = false
		t.l, t.r, t.fn, t.unary, t.word = skAtom, skAtom, s.atomFn, false, false
	}
	if t.l == skBin && t.r == skBin {
		switch s.prev {
		case skBoundary, skOpen, skGroupOpen, skBin, skRel, skPunct, skScript, skLeave:
			t.unary = true
		}
	}
	switch {
	case t.r == skLeave && t.l == skLeave && !t.unary:
		// Transparent for signs, except & which starts a cell.
		if s.line[t.start] == '&' {
			s.prev = skLeave
		}
	default:
		s.prev = t.r
	}
	if t.r == skAtom && t.l != skPrime && t.l != skGroupClose {
		s.atomFn = t.fn
	}
	if t.r == skScript {
		s.script = true
	}
	s.toks = append(s.toks, t)
}

// openGroup emits a { or [ group opener and pushes what its closer becomes.
func (s *spacingScanner) openGroup(p int) {
	o := spaceOwner{r: skLeave} // bare group: 1{,}5, {\bf x}y stay as written
	switch {
	case s.script:
		o = spaceOwner{r: skAtom, fn: s.atomFn}
		s.script = false
	case s.owner != nil:
		o = *s.owner
	}
	s.owner = nil
	s.stack = append(s.stack, o)
	s.emit(mathToken{start: p, end: p + 1, l: skGroupOpen, r: skGroupOpen})
}

// closeGroup emits a } or ] group closer.
func (s *spacingScanner) closeGroup(p int) {
	o := spaceOwner{r: skAtom}
	if n := len(s.stack); n > 0 {
		o = s.stack[n-1]
		s.stack = s.stack[:n-1]
	}
	s.emit(mathToken{start: p, end: p + 1, l: skGroupClose, r: o.r, fn: o.fn})
	if o.r == skAtom {
		s.atomFn = o.fn
	}
	s.owner = &o // \frac{a}{b}: the next group is another argument
}

func (s *spacingScanner) isMathByte(p int) bool {
	c := s.cls[p]
	return s.math[p] && (c == texscan.ClassMath || c == texscan.ClassMarkup)
}

// scanLine tokenizes one comment-stripped line.
func (s *spacingScanner) scanLine(line string, cls []texscan.Class, math []bool) []mathToken {
	s.line, s.cls, s.math, s.toks = line, cls, math, nil
	for p := 0; p < len(line); {
		b := line[p]
		if b == ' ' || b == '\t' {
			p++
			continue
		}
		if !math[p] {
			s.reset()
			p++
			continue
		}
		if !s.isMathByte(p) || cls[p] == texscan.ClassMarkup && isASCIILetter(b) {
			e := p + 1
			for e < len(line) && math[e] && line[e] != ' ' && (!s.isMathByte(e) || cls[e] == texscan.ClassMarkup && isASCIILetter(line[e])) {
				e++
			}
			s.toks = append(s.toks, mathToken{start: p, end: e, l: skLeave, r: skLeave})
			p = e
			continue
		}
		p = s.token(p)
	}
	return s.toks
}

// token scans the math token at p and returns its end.
func (s *spacingScanner) token(p int) int {
	line := s.line
	b := line[p]
	keep := s.owner
	s.owner = nil
	one := func(l, r spaceKind) int {
		s.emit(mathToken{start: p, end: p + 1, l: l, r: r})
		return p + 1
	}
	switch {
	case b == '\\':
		return s.command(p)
	case b == '$':
		e := p + 1
		if e < len(line) && line[e] == '$' {
			e++
		}
		s.emit(mathToken{start: p, end: e, l: skBoundary, r: skBoundary})
		return e
	case b == '{':
		s.owner = keep
		s.openGroup(p)
		return p + 1
	case b == '}':
		s.closeGroup(p)
		return p + 1
	case (b == '[' || b == ']') && s.cls[p] == texscan.ClassMarkup:
		if b == '[' {
			s.owner = keep
			s.openGroup(p)
		} else {
			s.closeGroup(p)
		}
		return p + 1
	case b == '^' || b == '_':
		if p+1 < len(line) && line[p+1] == b { // ^^ character notation
			e := min(p+4, len(line))
			s.emit(mathToken{start: p, end: e, l: skLeave, r: skLeave})
			return e
		}
		return one(skScript, skScript)
	case b == '\'':
		s.emit(mathToken{start: p, end: p + 1, l: skPrime, r: skAtom, fn: s.atomFn})
		return p + 1
	case b >= '0' && b <= '9':
		e := p + 1
		for e < len(line) && s.isMathByte(e) && (line[e] >= '0' && line[e] <= '9' ||
			line[e] == '.' && e+1 < len(line) && line[e+1] >= '0' && line[e+1] <= '9') {
			e++
		}
		s.emit(mathToken{start: p, end: e, l: skAtom, r: skAtom})
		return e
	case isASCIILetter(b):
		e := p + 1
		for e < len(line) && s.cls[e] == texscan.ClassMath && isASCIILetter(line[e]) {
			e++
		}
		s.emit(mathToken{start: p, end: e, l: skAtom, r: skAtom, fn: true, word: e-p >= 2})
		return e
	case b == '#' && p+1 < len(line) && line[p+1] >= '1' && line[p+1] <= '9':
		s.emit(mathToken{start: p, end: p + 2, l: skAtom, r: skAtom})
		return p + 2
	case b >= utf8.RuneSelf:
		r, size := utf8.DecodeRuneInString(line[p:])
		k, fn := skAtom, true
		if c, ok := texscan.UnicodeMathClasses[r]; ok && c != texscan.MathOp {
			k, fn = classKinds[c], false
		}
		s.emit(mathToken{start: p, end: p + size, l: k, r: k, fn: fn})
		return p + size
	}
	switch b {
	case '+', '-', '*':
		return one(skBin, skBin)
	case '=', '<', '>', ':':
		s.emit(mathToken{start: p, end: p + 1, l: skRel, r: skRel, relChar: true})
		return p + 1
	case '(', '[':
		return one(skOpen, skOpen)
	case ')', ']':
		return one(skClose, skClose)
	case ',', ';', '.', '!':
		return one(skPunct, skPunct)
	case '?', '@', '"':
		return one(skAtom, skAtom)
	}
	return one(skLeave, skLeave) // | / ~ & and anything unusual
}

// command scans the control sequence at p (a backslash).
func (s *spacingScanner) command(p int) int {
	line := s.line
	if p+1 >= len(line) {
		s.emit(mathToken{start: p, end: p + 1, l: skLeave, r: skLeave})
		return p + 1
	}
	if c := line[p+1]; !isASCIILetter(c) {
		e := p + 2
		k := skLeave
		switch c {
		case '\\':
			if e < len(line) && line[e] == '*' {
				e++
			}
			k = skBoundary
			s.emit(mathToken{start: p, end: e, l: k, r: k})
			s.owner = &spaceOwner{r: skBoundary} // \\[2pt]
			return e
		case '(', ')', '[', ']':
			k = skBoundary
		case '{':
			k = skOpen
		case '}':
			k = skClose
		case '#', '$', '%', '&', '_':
			k = skAtom
		}
		s.emit(mathToken{start: p, end: e, l: k, r: k})
		return e
	}
	e := p + 1
	for e < len(line) && isASCIILetter(line[e]) {
		e++
	}
	if e < len(line) && line[e] == '*' {
		e++ // never split \foo*
	}
	name := line[p+1 : e]
	switch {
	case name == "begin" || name == "end":
		s.emit(mathToken{start: p, end: e, l: skBoundary, r: skBoundary})
		s.owner = &spaceOwner{r: skBoundary}
		return e
	case spacingCmds[name]:
		s.emit(mathToken{start: p, end: e, l: skLeave, r: skLeave})
		s.owner = &spaceOwner{r: skLeave}
		return e
	case name == "limits" || name == "nolimits":
		s.emit(mathToken{start: p, end: e, l: skAttach, r: skAtom, fn: s.atomFn})
		return e
	}
	if k, ok := delimPrefixes[name]; ok {
		return s.delimited(p, e, k)
	}
	k, fn := skAtom, true
	if ck, ok := classKinds[s.ctx.MathClass(strings.TrimSuffix(name, "*"))]; ok {
		k, fn = ck, false
	}
	s.emit(mathToken{start: p, end: e, l: k, r: k, fn: fn})
	s.owner = &spaceOwner{r: k, fn: fn}
	return e
}

// delimited scans \left( \bigl\langle \not= … as one token of kind k (the
// delimiter's own kind for skLeave). Without a delimiter on this line the
// prefix alone is left untouched.
func (s *spacingScanner) delimited(p, e int, k spaceKind) int {
	line := s.line
	q := e
	for q < len(line) && (line[q] == ' ' || line[q] == '\t') {
		q++
	}
	if q >= len(line) || line[q] == '{' || line[q] == '}' || line[q] == '%' {
		s.emit(mathToken{start: p, end: e, l: skLeave, r: skLeave})
		return e
	}
	own := skLeave
	end := q + 1
	switch c := line[q]; {
	case c == '\\' && q+1 < len(line) && isASCIILetter(line[q+1]):
		end = q + 2
		for end < len(line) && isASCIILetter(line[end]) {
			end++
		}
		own = classKinds[s.ctx.MathClass(line[q+1:end])]
	case c == '\\' && q+1 < len(line):
		end = q + 2
		switch line[q+1] {
		case '{':
			own = skOpen
		case '}':
			own = skClose
		}
	case c >= utf8.RuneSelf:
		r, size := utf8.DecodeRuneInString(line[q:])
		end = q + size
		own = classKinds[texscan.UnicodeMathClasses[r]]
	case c == '(' || c == '[':
		own = skOpen
	case c == ')' || c == ']':
		own = skClose
	}
	if k == skLeave {
		k = own
		if k != skOpen && k != skClose {
			k = skLeave
		}
	}
	s.emit(mathToken{start: p, end: end, l: k, r: k})
	return end
}

// tight reports whether no space belongs between adjacent tokens a and b.
func tight(a, b mathToken) bool {
	switch {
	case a.r == skLeave || b.l == skLeave || a.r == skBoundary || b.l == skBoundary:
		return true
	case b.l == skScript || b.l == skPrime || b.l == skAttach || b.l == skGroupClose ||
		b.l == skClose || b.l == skPunct || b.l == skGroupOpen:
		return true
	case a.r == skScript || a.r == skGroupOpen || a.r == skOpen || a.unary:
		return true
	case a.relChar && b.relChar: // := =:
		return true
	case b.l == skOpen: // f(x) \sin(x), but 2 (x) and ) (
		return a.r == skAtom && a.fn
	}
	return false
}

// spacingFinding is a missing space (at insert, before byte insert) or a
// bare word (word true, insert = start, end).
type spacingFinding struct {
	insert, end int
	word        bool
}

// mathSpacingFindings returns per line the missing spaces between adjacent
// math tokens and the runs of 2+ letters (bare words, not fixed: x y or
// \max is the author's call).
func mathSpacingFindings(ctx *Context, stripped []string) [][]spacingFinding {
	classes, maths := texscan.ClassifyMath(stripped)
	s := &spacingScanner{ctx: ctx}
	s.reset()
	out := make([][]spacingFinding, len(stripped))
	for i, line := range stripped {
		toks := s.scanLine(line, classes[i], maths[i])
		for k, t := range toks {
			if t.word {
				out[i] = append(out[i], spacingFinding{insert: t.start, end: t.end, word: true})
			}
			if k > 0 && toks[k-1].end == t.start && !tight(toks[k-1], t) {
				out[i] = append(out[i], spacingFinding{insert: t.start})
			}
		}
	}
	return out
}

// checkMathTokenSpacing flags adjacent math tokens without a space between
// them (a+b, 2x, \alpha\beta, f(x,y)) and bare words in math ($pauli$).
func checkMathTokenSpacing(ctx *Context, path string, lines []string) []Diagnostic {
	stripped := stripComments(lines)
	var diags []Diagnostic
	for i, fs := range mathSpacingFindings(ctx, stripped) {
		for _, f := range fs {
			msg := fmt.Sprintf("missing space in math at column %d", f.insert+1)
			if f.word {
				msg = fmt.Sprintf("bare word %q in math mode", stripped[i][f.insert:f.end])
			}
			diags = append(diags, Diagnostic{File: path, Line: i + 1, Message: msg})
		}
	}
	return diags
}

// fixMathTokenSpacing inserts the missing spaces. Bare words are left alone.
func fixMathTokenSpacing(ctx *Context, path string, lines []string) ([]string, bool) {
	stripped := stripComments(lines)
	changed := false
	for i, fs := range mathSpacingFindings(ctx, stripped) {
		line := stripped[i]
		did := false
		for k := len(fs) - 1; k >= 0; k-- {
			if !fs[k].word {
				line = line[:fs[k].insert] + " " + line[fs[k].insert:]
				did = true
			}
		}
		if did {
			lines[i] = line + lines[i][len(stripped[i]):]
			changed = true
		}
	}
	return lines, changed
}
