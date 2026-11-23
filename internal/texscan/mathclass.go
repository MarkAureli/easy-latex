package texscan

import "maps"

// MathClass is the TeX math class of a symbol: what kind of atom it forms.
type MathClass string

const (
	MathOrd   MathClass = "ord"   // ordinary (\alpha, \infty; also class 7)
	MathOp    MathClass = "op"    // large operator (\sum, \int)
	MathBin   MathClass = "bin"   // binary operator (\cdot, \times)
	MathRel   MathClass = "rel"   // relation (\le, \in, \to)
	MathOpen  MathClass = "open"  // opening delimiter (\langle)
	MathClose MathClass = "close" // closing delimiter (\rangle)
	MathPunct MathClass = "punct" // punctuation (\colon)
)

// MathClassFromDigit maps TeX's numeric math class (0–7) to a MathClass.
func MathClassFromDigit(d int) (MathClass, bool) {
	switch d {
	case 0, 7:
		return MathOrd, true
	case 1:
		return MathOp, true
	case 2:
		return MathBin, true
	case 3:
		return MathRel, true
	case 4:
		return MathOpen, true
	case 5:
		return MathClose, true
	case 6:
		return MathPunct, true
	}
	return "", false
}

// mathClassNames groups the built-in classes for readability.
var mathClassNames = map[MathClass][]string{
	MathBin: {
		"pm", "mp", "times", "div", "cdot", "ast", "star", "circ", "bullet",
		"cap", "cup", "sqcap", "sqcup", "vee", "wedge", "lor", "land", "setminus",
		"smallsetminus", "oplus", "ominus", "otimes", "oslash", "odot", "uplus",
		"amalg", "dagger", "ddagger", "wr", "diamond", "bigtriangleup",
		"bigtriangledown", "triangleleft", "triangleright", "lhd", "rhd", "unlhd",
		"unrhd", "boxplus", "boxminus", "boxtimes", "boxdot", "ltimes", "rtimes",
		"dotplus", "centerdot", "intercal", "barwedge", "veebar", "curlywedge",
		"curlyvee", "divideontimes", "circledast", "circledcirc", "circleddash",
		"Cap", "Cup", "doublecap", "doublecup",
	},
	MathRel: {
		"le", "leq", "ge", "geq", "ne", "neq", "equiv", "sim", "simeq", "approx",
		"cong", "propto", "prec", "succ", "preceq", "succeq", "ll", "gg", "lll",
		"ggg", "subset", "supset", "subseteq", "supseteq", "subsetneq",
		"supsetneq", "nsubseteq", "nsupseteq", "sqsubset", "sqsupset",
		"sqsubseteq", "sqsupseteq", "in", "ni", "notin", "owns", "vdash", "dashv",
		"models", "perp", "mid", "parallel", "nmid", "nparallel", "smile", "frown",
		"asymp", "doteq", "bowtie", "Join", "leqslant", "geqslant", "nleq", "ngeq",
		"nless", "ngtr", "lneq", "gneq", "lesssim", "gtrsim", "approxeq",
		"thicksim", "thickapprox", "backsim", "eqsim", "triangleq",
		"trianglelefteq", "trianglerighteq", "vDash", "Vdash", "Vvdash", "nvdash",
		"nvDash", "to", "gets", "rightarrow", "leftarrow", "leftrightarrow",
		"Rightarrow", "Leftarrow", "Leftrightarrow", "longrightarrow",
		"longleftarrow", "longleftrightarrow", "Longrightarrow", "Longleftarrow",
		"Longleftrightarrow", "mapsto", "longmapsto", "hookrightarrow",
		"hookleftarrow", "uparrow", "downarrow", "updownarrow", "Uparrow",
		"Downarrow", "Updownarrow", "nearrow", "searrow", "swarrow", "nwarrow",
		"rightharpoonup", "rightharpoondown", "leftharpoonup", "leftharpoondown",
		"rightleftharpoons", "twoheadrightarrow", "twoheadleftarrow",
		"rightsquigarrow", "leadsto", "iff", "implies", "impliedby", "coloneqq",
		"eqqcolon", "coloneq", "eqcolon", "Coloneqq", "xrightarrow", "xleftarrow",
	},
	MathOpen:  {"langle", "lceil", "lfloor", "lbrace", "lbrack", "lgroup", "lmoustache", "lvert", "lVert", "llbracket"},
	MathClose: {"rangle", "rceil", "rfloor", "rbrace", "rbrack", "rgroup", "rmoustache", "rvert", "rVert", "rrbracket"},
	MathPunct: {"colon", "ldotp", "cdotp"},
}

// BuiltinMathClasses holds the math class of common non-ordinary symbol
// commands. It backs the compile probe (internal/cmdprobe) for symbols the
// probe cannot classify (macros such as \neq = \not= or \iff) and stands in
// for it before the first compile. Absent names are ordinary.
var BuiltinMathClasses = func() map[string]MathClass {
	out := map[string]MathClass{}
	for c, names := range mathClassNames {
		for _, n := range names {
			out[n] = c
		}
	}
	return out
}()

// UnicodeMathClasses holds the class of common non-ordinary Unicode math
// characters, typed directly (a≤b) or reported by unicode-math's probe
// meaning ("the character ∈").
var UnicodeMathClasses = map[rune]MathClass{
	'±': MathBin, '∓': MathBin, '×': MathBin, '÷': MathBin, '⋅': MathBin, '·': MathBin,
	'∘': MathBin, '∙': MathBin, '∩': MathBin, '∪': MathBin, '∧': MathBin, '∨': MathBin,
	'∖': MathBin, '⊕': MathBin, '⊖': MathBin, '⊗': MathBin, '⊙': MathBin, '−': MathBin,
	'≤': MathRel, '≥': MathRel, '≠': MathRel, '≡': MathRel, '∼': MathRel, '≃': MathRel,
	'≈': MathRel, '≅': MathRel, '∝': MathRel, '≪': MathRel, '≫': MathRel, '⊂': MathRel,
	'⊃': MathRel, '⊆': MathRel, '⊇': MathRel, '⊊': MathRel, '⊋': MathRel, '∈': MathRel,
	'∉': MathRel, '∋': MathRel, '⊢': MathRel, '⊨': MathRel, '⊥': MathRel, '∣': MathRel,
	'∥': MathRel, '→': MathRel, '←': MathRel, '↔': MathRel, '⇒': MathRel, '⇐': MathRel,
	'⇔': MathRel, '↦': MathRel, '⟶': MathRel, '⟹': MathRel, '⟺': MathRel, '≔': MathRel,
	'⟨': MathOpen, '⌈': MathOpen, '⌊': MathOpen, '⟦': MathOpen,
	'⟩': MathClose, '⌉': MathClose, '⌋': MathClose, '⟧': MathClose,
	'∑': MathOp, '∏': MathOp, '∫': MathOp, '∮': MathOp, '⋃': MathOp, '⋂': MathOp,
}

// StoredMathClasses returns BuiltinMathClasses overlaid by the class table at
// path (.el/mathclass.json, written from the compile probe). A missing file
// yields the built-in table alone.
func StoredMathClasses(path string) map[string]MathClass {
	out := maps.Clone(BuiltinMathClasses)
	maps.Copy(out, LoadTable[MathClass](path))
	return out
}
