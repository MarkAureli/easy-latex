package texscan

import "strings"

// Class is the role of one source byte, shared by all checks: prose (in one
// of four sentence contexts), math, markup, opaque or verbatim.
type Class uint8

const (
	ClassText       Class = iota // running prose: body text, inline prose arguments (\emph{…})
	ClassStandalone              // prose with its own sentence context (\footnote, \caption)
	ClassHeading                 // prose without sentence punctuation (\section, \item[…], TikZ node text)
	ClassCell                    // prose in table, algorithm or bibliography cells
	ClassMath
	ClassMarkup   // command names, structural braces/brackets, math and env delimiters
	ClassOpaque   // identifiers, paths, lengths, options, definitions, graphics code
	ClassVerbatim // verbatim environments and \verb
)

// IsProse reports whether c is one of the prose classes.
func (c Class) IsProse() bool { return c <= ClassCell }

// classInline marks a prose argument that continues the surrounding sentence
// context (\emph inside a footnote stays standalone). Resolved on push; in
// math it is running text (labels belong in \mathrm, which is opaque).
const classInline Class = 255

// arg describes one argument of a known command.
type arg struct {
	opt   bool  // [optional]; unlisted options are always opaque
	class Class // content class (classInline, a prose class, ClassMath or ClassOpaque)
	code  bool  // TikZ code: {…} body in code mode, […] options with prose keys
}

var (
	argInline     = arg{class: classInline}
	argOpaque     = arg{class: ClassOpaque}
	argHeading    = arg{class: ClassHeading}
	argStandalone = arg{class: ClassStandalone}
	optHeading    = arg{opt: true, class: ClassHeading}
	optStandalone = arg{opt: true, class: ClassStandalone}
)

// commands lists the commands whose arguments are not all opaque (or, in
// math, not all math). Arguments of any other command are opaque in text and
// code, and math in math. Starred forms share the entry.
var commands = map[string][]arg{
	// Font / emphasis
	"emph": {argInline}, "textbf": {argInline}, "textit": {argInline}, "textrm": {argInline},
	"textsf": {argInline}, "texttt": {argInline}, "textsc": {argInline}, "textsl": {argInline},
	"textup": {argInline}, "textmd": {argInline}, "textnormal": {argInline},
	"textsuperscript": {argInline}, "textsubscript": {argInline}, "underline": {argInline},
	"uline": {argInline}, "sout": {argInline}, "hl": {argInline}, "highlight": {argInline},
	"enquote": {argInline}, "text": {argInline}, "intertext": {argInline},
	// Boxes
	"mbox": {argInline}, "hbox": {argInline}, "fbox": {argInline}, "makebox": {argInline},
	"framebox": {argInline}, "parbox": {argOpaque, argInline}, "raisebox": {argOpaque, argInline},
	"scalebox": {argOpaque, argInline}, "rotatebox": {argOpaque, argInline},
	"colorbox": {argOpaque, argInline}, "fcolorbox": {argOpaque, argOpaque, argInline},
	"resizebox": {argOpaque, argOpaque, argInline},
	// Colour / links / language
	"textcolor": {argOpaque, argInline}, "href": {argOpaque, argInline},
	"hyperlink": {argOpaque, argInline}, "hypertarget": {argOpaque, argInline},
	"foreignlanguage": {argOpaque, argInline},
	// Structure
	"part": {optHeading, argHeading}, "chapter": {optHeading, argHeading},
	"section": {optHeading, argHeading}, "subsection": {optHeading, argHeading},
	"subsubsection": {optHeading, argHeading}, "paragraph": {optHeading, argHeading},
	"subparagraph": {optHeading, argHeading}, "item": {optHeading},
	"caption": {optStandalone, argStandalone}, "captionof": {argOpaque, argStandalone},
	"footnote": {argStandalone}, "footnotetext": {argStandalone}, "marginpar": {argStandalone},
	"thanks": {argStandalone},
	// Front matter
	"title": {optHeading, argHeading}, "subtitle": {argHeading}, "author": {optHeading, argHeading},
	"date": {argHeading}, "institute": {argHeading}, "affiliation": {argHeading},
	"keywords": {argHeading},
	// Tables
	"multicolumn": {argOpaque, argOpaque, argInline}, "multirow": {argOpaque, argOpaque, argInline},
	// pgfplots legends
	"addlegendentry": {argHeading}, "legend": {argHeading},
	// Inline TikZ
	"tikz": {{opt: true, code: true}, {code: true}},
	// Names, keys and lengths: opaque in math, too (math defaults to math).
	"label": {argOpaque}, "tag": {argOpaque}, "ref": {argOpaque}, "eqref": {argOpaque},
	"hspace": {argOpaque}, "vspace": {argOpaque}, "operatorname": {argOpaque},
	"mathrm": {argOpaque}, "mathsf": {argOpaque}, "mathtt": {argOpaque}, "mathit": {argOpaque},
	// Line break: \\[2pt]
	"\\": {{opt: true, class: ClassOpaque}},
}

// defCommands are TeX primitives defining a macro from an unbraced name and
// #-parameters: \def\foo#1{body}.
var defCommands = map[string]bool{"def": true, "gdef": true, "edef": true, "xdef": true}

// envMode is how an environment's body is classified.
type envMode uint8

const (
	envInherit envMode = iota
	envMath
	envVerbatim
	envCode  // TikZ/PGF code: bare text opaque, node text and some option values prose
	envCells // prose cells, not sentences
)

var environments = map[string]envMode{
	"equation": envMath, "equation*": envMath, "align": envMath, "align*": envMath,
	"gather": envMath, "gather*": envMath, "multline": envMath, "multline*": envMath,
	"eqnarray": envMath, "eqnarray*": envMath, "displaymath": envMath, "math": envMath,
	"flalign": envMath, "flalign*": envMath, "alignat": envMath, "alignat*": envMath,
	"tikzcd": envMath, "dmath": envMath, "dmath*": envMath, "dgroup": envMath, "dgroup*": envMath,
	"darray": envMath, "darray*": envMath,

	"verbatim": envVerbatim, "Verbatim": envVerbatim, "BVerbatim": envVerbatim,
	"LVerbatim": envVerbatim, "lstlisting": envVerbatim, "minted": envVerbatim,
	"comment": envVerbatim, "alltt": envVerbatim,

	"tikzpicture": envCode, "pgfpicture": envCode, "circuitikz": envCode, "forest": envCode,
	"pspicture": envCode, "scope": envCode, "axis": envCode, "semilogxaxis": envCode,
	"semilogyaxis": envCode, "loglogaxis": envCode, "quantikz": envCode,

	"tabular": envCells, "tabular*": envCells, "tabularx": envCells, "tabulary": envCells,
	"longtable": envCells, "algorithm": envCells, "algorithm*": envCells,
	"algorithmic": envCells, "thebibliography": envCells, "IEEEkeywords": envCells,
}

// IsVerbatimEnv reports whether env's body is verbatim.
func IsVerbatimEnv(env string) bool { return environments[env] == envVerbatim }

// IsMathEnv reports whether env's body is display math.
func IsMathEnv(env string) bool { return environments[env] == envMath }

// codeProseKeys are option keys whose values are prose inside TikZ/PGF code
// (pgfplots axis labels and legends).
var codeProseKeys = map[string]bool{
	"xlabel": true, "ylabel": true, "zlabel": true, "title": true, "legend entries": true,
}

type mode uint8

const (
	mText    mode = iota // prose of frame.class; commands, math and groups parsed
	mMath                // math; unknown command arguments stay math
	mCode                // TikZ code: bytes opaque; commands and `node` parsed
	mOpaque              // everything opaque; only braces/brackets nest
	mOptions             // […] option list: opaque, optionally with prose keys
	mVerbEnv             // verbatim environment body
	mVerb                // \verb|…|
)

// Closers of a frame (besides an environment name).
const (
	closeBrace = "}"
	closeBrack = "]"
	closeValue = ","   // unbraced prose option value: ends before `,` or `]`
	closeEnv   = "env" // ends at \end{frame.env}
)

type frame struct {
	mode   mode
	class  Class
	close  string // closeBrace, closeBrack, closeValue, closeEnv, "$", "$$", `\)`, `\]`, or the \verb delimiter
	env    string
	depth  int    // nested plain {…} groups
	bdepth int    // nested […] inside a closeBrack frame
	after  *await // argument parsing to resume once this frame closes

	math bool // inside math (this frame or an ancestor is mMath)

	keys bool   // mOptions: values of codeProseKeys are prose
	key  []byte // mOptions: key read so far at the top level
}

// await is the argument parsing pending after a command or \begin{env}.
type await struct {
	args      []arg // known command: remaining specs
	rest      bool  // unknown command: every consecutive group gets restClass
	restClass Class
	immediate bool // env args: groups must follow without whitespace
	codeOpts  bool // […] options get prose-key scanning
	node      bool // TikZ node: options, (name), `at (x,y)`, then {prose}
	def       int  // \def: 1 = expect \name, 2 = parameters, then one opaque body
}

// crossesLines reports whether the pending parsing may continue on the next
// line: only for a known command still expecting arguments (LaTeX skips the
// line break when looking for them: \\ + newline + [1em], \section + newline
// + {Title}). Groups on the next line after an unknown command (\vfill +
// newline + {\small …}) or after env args are not taken as arguments.
func (aw *await) crossesLines() bool {
	return !aw.rest && !aw.immediate && !aw.node && aw.def == 0 && len(aw.args) > 0
}

type lexer struct {
	stack []frame
	aw    *await
	line  string
	out   []Class
	math  []bool // per byte: inside math (delimiters excluded)
	i     int
}

// Classify assigns a Class to every byte of the given comment-stripped
// lines (see StripComment). State carries across lines; a blank line ends
// pending argument parsing and open option lists.
func Classify(lines []string) [][]Class {
	classes, _ := ClassifyMath(lines)
	return classes
}

// ClassifyMath is Classify that also reports, per byte, whether it lies
// inside math — including markup and opaque arguments there (\alpha,
// \text{…}) but not the math delimiters themselves ($, \[, \begin{equation}).
func ClassifyMath(lines []string) ([][]Class, [][]bool) {
	infos := Analyze(lines)
	classes, maths := make([][]Class, len(infos)), make([][]bool, len(infos))
	for i, in := range infos {
		classes[i], maths[i] = in.Classes, in.Math
	}
	return classes, maths
}

// LineInfo is the classification of one line.
type LineInfo struct {
	Classes []Class // per byte
	Math    []bool  // per byte: inside math (see ClassifyMath)
	// End is the class of the innermost context still open at the end of the
	// line: a prose class, ClassMath, ClassOpaque (inside an opaque argument,
	// option list or graphics code) or ClassVerbatim.
	End Class
	// OpensGroup reports that a {…} or […] group opened on this line is still
	// open at its end (the group's content continues on the next line).
	OpensGroup bool
}

// Analyze classifies lines like Classify and adds per-line context.
func Analyze(lines []string) []LineInfo {
	lx := &lexer{stack: []frame{{mode: mText, class: ClassText}}}
	res := make([]LineInfo, len(lines))
	for li, line := range lines {
		if strings.TrimSpace(line) == "" {
			lx.paragraph()
		}
		lx.line, lx.out, lx.math, lx.i = line, make([]Class, len(line)), make([]bool, len(line)), 0
		minLevel := lx.groupLevel()
		for lx.i < len(line) {
			lx.step()
			minLevel = min(minLevel, lx.groupLevel())
		}
		if lx.top().mode == mVerb { // \verb cannot span lines
			lx.pop()
		}
		if lx.aw != nil && !lx.aw.crossesLines() {
			lx.aw = nil
		}
		res[li] = LineInfo{Classes: lx.out, Math: lx.math, End: lx.top().class, OpensGroup: lx.groupLevel() > minLevel}
	}
	return res
}

// groupLevel is the current nesting of {…} / […] groups (argument frames and
// plain groups; environments and math delimiters do not count).
func (lx *lexer) groupLevel() int {
	n := 0
	for i := range lx.stack {
		f := &lx.stack[i]
		n += f.depth
		if f.close == closeBrace || f.close == closeBrack || f.close == closeValue {
			n++
		}
	}
	return n
}

func (lx *lexer) top() *frame { return &lx.stack[len(lx.stack)-1] }

func (lx *lexer) mark(n int, c Class) {
	m := lx.top().math
	for j := 0; j < n && lx.i < len(lx.line); j++ {
		lx.out[lx.i] = c
		lx.math[lx.i] = m
		lx.i++
	}
}

func (lx *lexer) push(f frame) {
	f.math = f.mode == mMath || lx.top().math
	lx.stack = append(lx.stack, f)
}

// pop closes the top frame and resumes its pending argument parsing.
func (lx *lexer) pop() {
	if len(lx.stack) == 1 {
		return
	}
	f := lx.stack[len(lx.stack)-1]
	lx.stack = lx.stack[:len(lx.stack)-1]
	lx.aw = f.after
}

// popTo closes frames down to and including the innermost one matching ok.
// Reports false (and changes nothing) if there is none.
func (lx *lexer) popTo(ok func(*frame) bool) bool {
	for k := len(lx.stack) - 1; k > 0; k-- {
		if ok(&lx.stack[k]) {
			lx.stack = lx.stack[:k+1]
			lx.pop()
			return true
		}
	}
	return false
}

func (lx *lexer) paragraph() {
	lx.aw = nil
	for {
		switch t := lx.top(); {
		case t.mode == mOptions, t.close == closeValue:
			lx.pop()
			lx.aw = nil
			continue
		}
		return
	}
}

// frameFor builds the frame for an argument with the given spec.
func (lx *lexer) frameFor(a arg, close string) frame {
	if a.code {
		if close == closeBrack {
			return frame{mode: mOptions, class: ClassOpaque, close: close, keys: true}
		}
		return frame{mode: mCode, class: ClassOpaque, close: close}
	}
	switch c := a.class; {
	case c == ClassMath:
		return frame{mode: mMath, class: ClassMath, close: close}
	case c == ClassOpaque:
		if close == closeBrack {
			return frame{mode: mOptions, class: ClassOpaque, close: close}
		}
		return frame{mode: mOpaque, class: ClassOpaque, close: close}
	case c == classInline:
		c = ClassText // \text{…} inside math is running text
		if t := lx.top().class; t.IsProse() {
			c = t
		}
		return frame{mode: mText, class: c, close: close}
	default:
		return frame{mode: mText, class: c, close: close}
	}
}

func (lx *lexer) step() {
	t := lx.top()
	switch t.mode {
	case mVerbEnv:
		if name, n := matchEndEnvName(lx.line, lx.i); n > 0 && name == t.env {
			lx.mark(n, ClassMarkup)
			lx.pop()
			return
		}
		lx.mark(1, ClassVerbatim)
		return
	case mVerb:
		if lx.line[lx.i] == t.close[0] {
			lx.mark(1, ClassMarkup)
			lx.pop()
			return
		}
		lx.mark(1, ClassVerbatim)
		return
	}
	if lx.aw != nil && lx.stepAwait() {
		return
	}
	switch t.mode {
	case mOpaque, mOptions:
		lx.stepOpaque()
	default:
		lx.stepParsed()
	}
}

// stepAwait consumes the next byte as part of pending argument parsing.
// Reports false when parsing ended and the byte must be processed normally.
func (lx *lexer) stepAwait() bool {
	aw := lx.aw
	c := lx.line[lx.i]
	cls := lx.top().class
	if aw.def > 0 {
		switch {
		case c == ' ' || c == '\t':
			lx.mark(1, cls)
		case aw.def == 1 && c == '\\':
			_, n := readMacroName(lx.line, lx.i)
			lx.mark(max(n, 1), ClassMarkup)
			aw.def = 2
		case aw.def == 2 && c == '#':
			lx.mark(2, ClassOpaque)
		case c == '{':
			lx.mark(1, ClassMarkup)
			lx.push(frame{mode: mOpaque, class: ClassOpaque, close: closeBrace})
			lx.aw = nil
		default:
			lx.aw = nil
			return false
		}
		return true
	}
	if aw.node {
		switch {
		case c == ' ' || c == '\t':
			lx.mark(1, ClassOpaque)
		case c == '[':
			lx.mark(1, ClassMarkup)
			f := frame{mode: mOptions, class: ClassOpaque, close: closeBrack, keys: true, after: aw}
			lx.aw = nil
			lx.push(f)
		case c == '(':
			lx.skipParens()
		case strings.HasPrefix(lx.line[lx.i:], "at") && !isWordByte(lx.line, lx.i+2):
			lx.mark(2, ClassOpaque)
		case c == '{':
			lx.mark(1, ClassMarkup)
			lx.aw = nil
			lx.push(frame{mode: mText, class: ClassHeading, close: closeBrace})
		default:
			lx.aw = nil
			return false
		}
		return true
	}
	if c == ' ' || c == '\t' {
		if aw.immediate {
			lx.aw = nil
			return false
		}
		lx.mark(1, cls)
		return true
	}
	if c != '[' && c != '{' {
		lx.aw = nil
		return false
	}
	var f frame
	next := aw
	switch {
	case aw.rest:
		if c == '[' && aw.restClass == ClassMath {
			lx.aw = nil // \sqrt[3], \left[: plain math
			return false
		}
		close := closeBrace
		if c == '[' {
			close = closeBrack
		}
		f = lx.frameFor(arg{class: aw.restClass, code: aw.codeOpts && c == '['}, close)
	case c == '[':
		if len(aw.args) == 0 {
			lx.aw = nil
			return false
		}
		if aw.args[0].opt {
			f = lx.frameFor(aw.args[0], closeBrack)
			next = &await{args: aw.args[1:]}
		} else {
			f = lx.frameFor(arg{class: ClassOpaque, code: aw.codeOpts}, closeBrack)
		}
	default: // '{'
		args := aw.args
		for len(args) > 0 && args[0].opt {
			args = args[1:]
		}
		if len(args) == 0 {
			lx.aw = nil
			return false
		}
		f = lx.frameFor(args[0], closeBrace)
		next = &await{args: args[1:]}
	}
	if !next.rest && !next.immediate && len(next.args) == 0 {
		next = nil
	}
	f.after = next
	lx.aw = nil
	lx.mark(1, ClassMarkup)
	lx.push(f)
	return true
}

// skipParens marks a balanced (…) group (TikZ coordinates, node names) as
// opaque, up to the end of the line.
func (lx *lexer) skipParens() {
	depth := 0
	for lx.i < len(lx.line) {
		c := lx.line[lx.i]
		lx.mark(1, ClassOpaque)
		if c == '(' {
			depth++
		} else if c == ')' {
			depth--
			if depth == 0 {
				return
			}
		}
	}
}

func isWordByte(s string, i int) bool {
	return i >= 0 && i < len(s) && (isLetter(s[i]) || s[i] == '@')
}

// stepOpaque handles mOpaque and mOptions frames: only nesting is tracked.
func (lx *lexer) stepOpaque() {
	t := lx.top()
	c := lx.line[lx.i]
	switch {
	case c == '\\':
		lx.mark(2, ClassOpaque)
	case c == '{':
		if t.keys && t.depth == 0 && t.bdepth == 0 && lx.proseKey() {
			lx.mark(1, ClassMarkup)
			lx.push(frame{mode: mText, class: ClassHeading, close: closeBrace})
			return
		}
		t.depth++
		lx.mark(1, ClassOpaque)
	case c == '}':
		if t.depth > 0 {
			t.depth--
			lx.mark(1, ClassOpaque)
			return
		}
		lx.closeBrace()
	case t.close == closeBrack && c == '[' && t.depth == 0:
		t.bdepth++
		lx.mark(1, ClassOpaque)
	case t.close == closeBrack && c == ']' && t.depth == 0:
		if t.bdepth > 0 {
			t.bdepth--
			lx.mark(1, ClassOpaque)
			return
		}
		lx.mark(1, ClassMarkup)
		lx.pop()
	default:
		if t.keys && t.depth == 0 && t.bdepth == 0 {
			switch c {
			case ',':
				t.key = t.key[:0]
			case '=':
				// The key is complete; its value starts after `=`.
				t.key = append(t.key, '=')
			case ' ', '\t':
				if len(t.key) > 0 && t.key[len(t.key)-1] == '=' {
					break // spaces before the value
				}
				t.key = append(t.key, c)
			default:
				if lx.proseKey() {
					// Unbraced value: prose up to the next `,` or `]`.
					lx.push(frame{mode: mText, class: ClassHeading, close: closeValue})
					return
				}
				t.key = append(t.key, c)
			}
		}
		lx.mark(1, ClassOpaque)
	}
}

// proseKey reports whether the option key just completed (`key=`) is one of
// codeProseKeys, and resets the key.
func (lx *lexer) proseKey() bool {
	t := lx.top()
	k := string(t.key)
	if !strings.HasSuffix(k, "=") {
		return false
	}
	t.key = t.key[:0]
	return codeProseKeys[strings.TrimSpace(strings.TrimSuffix(k, "="))]
}

// closeBrace handles a `}` that closes no plain group of the top frame.
func (lx *lexer) closeBrace() {
	lx.mark(1, ClassMarkup)
	lx.popTo(func(f *frame) bool { return f.close == closeBrace && f.depth == 0 })
}

// stepParsed handles mText, mMath and mCode frames.
func (lx *lexer) stepParsed() {
	t := lx.top()
	line, i := lx.line, lx.i
	c := line[i]

	if t.close == closeValue && t.depth == 0 && (c == ',' || c == ']') {
		lx.pop() // the option list handles the `,` / `]`
		return
	}
	switch c {
	case '\\':
		lx.command()
		return
	case '$':
		if t.mode == mCode {
			break // TikZ calc syntax ($(a)!0.5!(b)$), not math
		}
		closer := "$"
		if strings.HasPrefix(line[i:], "$$") {
			closer = "$$"
		}
		if t.mode != mMath {
			lx.mark(len(closer), ClassMarkup)
			lx.push(frame{mode: mMath, class: ClassMath, close: closer})
		} else {
			lx.popTo(func(f *frame) bool { return f.close == closer })
			lx.mark(len(closer), ClassMarkup)
		}
		return
	case '{':
		t.depth++
		lx.mark(1, ClassMarkup)
		return
	case '}':
		if t.depth > 0 {
			t.depth--
			lx.mark(1, ClassMarkup)
			return
		}
		lx.closeBrace()
		return
	case '[':
		if t.close == closeBrack && t.depth == 0 {
			t.bdepth++
		}
	case ']':
		if t.close == closeBrack && t.depth == 0 {
			if t.bdepth > 0 {
				t.bdepth--
			} else {
				lx.mark(1, ClassMarkup)
				lx.pop()
				return
			}
		}
	case '&':
		lx.mark(1, ClassMarkup)
		return
	}
	if t.mode == mCode {
		if c == 'n' && strings.HasPrefix(line[i:], "node") && !isWordByte(line, i-1) && !isWordByte(line, i+4) {
			lx.mark(4, ClassOpaque)
			lx.aw = &await{node: true}
			return
		}
		lx.mark(1, ClassOpaque)
		return
	}
	lx.mark(1, t.class)
}

// accentCmds are single-character accent macros (\'e); the accented letter
// stays prose.
const accentCmds = "'`^\"~=."

// command handles a backslash in mText, mMath or mCode.
func (lx *lexer) command() {
	t := lx.top()
	line, i := lx.line, lx.i
	if i+1 >= len(line) {
		lx.mark(1, ClassMarkup)
		return
	}
	nx := line[i+1]
	switch {
	case nx == '\\':
		n := 2
		if i+2 < len(line) && line[i+2] == '*' {
			n = 3
		}
		lx.mark(n, ClassMarkup)
		lx.aw = &await{args: commands["\\"]}
		return
	case nx == '(' || nx == '[':
		if t.mode == mMath {
			lx.mark(2, ClassMarkup)
			return
		}
		closer := `\)`
		if nx == '[' {
			closer = `\]`
		}
		lx.mark(2, ClassMarkup)
		lx.push(frame{mode: mMath, class: ClassMath, close: closer})
		return
	case nx == ')' || nx == ']':
		closer := `\` + string(nx)
		lx.popTo(func(f *frame) bool { return f.close == closer })
		lx.mark(2, ClassMarkup)
		return
	case strings.IndexByte(accentCmds, nx) >= 0:
		lx.mark(1, ClassMarkup)
		return
	case !isLetter(nx) && nx != '@':
		lx.mark(2, ClassMarkup)
		return
	}

	if name, n := matchBeginEnvName(line, i); n > 0 {
		lx.mark(n, ClassMarkup)
		lx.begin(name)
		return
	}
	if name, n := matchEndEnvName(line, i); n > 0 {
		lx.popTo(func(f *frame) bool { return f.close == closeEnv && f.env == name })
		lx.mark(n, ClassMarkup)
		return
	}
	name, n := readMacroName(line, i)
	lx.mark(n, ClassMarkup)
	switch {
	case name == "verb" || name == "lstinline":
		if lx.i < len(line) && line[lx.i] != '{' && line[lx.i] != '[' {
			lx.push(frame{mode: mVerb, close: string(line[lx.i])})
			lx.mark(1, ClassMarkup)
		} else {
			lx.aw = &await{rest: true, restClass: ClassOpaque}
		}
	case defCommands[name] && t.mode != mMath:
		lx.aw = &await{def: 1}
	case t.mode == mCode && name == "node":
		lx.aw = &await{node: true}
	default:
		if args, ok := commands[name]; ok {
			lx.aw = &await{args: args}
		} else if t.mode == mMath {
			lx.aw = &await{rest: true, restClass: ClassMath}
		} else {
			lx.aw = &await{rest: true, restClass: ClassOpaque, codeOpts: t.mode == mCode}
		}
	}
}

// begin pushes the frame of environment name, whose \begin{name} was just
// marked, and arms parsing of its immediately following arguments (opaque).
func (lx *lexer) begin(name string) {
	t := lx.top()
	f := frame{mode: t.mode, class: t.class, close: closeEnv, env: name}
	code := false
	switch environments[name] {
	case envMath:
		f.mode, f.class = mMath, ClassMath
	case envVerbatim:
		f.mode, f.class = mVerbEnv, ClassVerbatim
		lx.push(f)
		return
	case envCode:
		f.mode, f.class = mCode, ClassOpaque
		code = true
	case envCells:
		f.mode, f.class = mText, ClassCell
	}
	lx.push(f)
	lx.aw = &await{rest: true, restClass: ClassOpaque, immediate: true, codeOpts: code}
}
