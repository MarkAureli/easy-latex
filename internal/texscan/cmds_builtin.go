package texscan

// BuiltinCmdArgs holds the mandatory-argument counts of common LaTeX kernel
// and package commands. It backs the compile probe (internal/cmdprobe) for
// commands the probe cannot resolve (helpers such as \genfrac, \mathaccent,
// \@startsection) and stands in for it before the first compile. Commands
// taking no mandatory argument are omitted; usage decides those.
var BuiltinCmdArgs = map[string]int{
	// Fractions, binomials, roots.
	"frac": 2, "dfrac": 2, "tfrac": 2, "cfrac": 2,
	"binom": 2, "dbinom": 2, "tbinom": 2, "genfrac": 6,
	"sqrt": 1,

	// Math accents and over/under decorations.
	"hat": 1, "widehat": 1, "check": 1, "tilde": 1, "widetilde": 1,
	"acute": 1, "grave": 1, "dot": 1, "ddot": 1, "dddot": 1, "ddddot": 1,
	"breve": 1, "bar": 1, "vec": 1, "mathring": 1,
	"overline": 1, "underline": 1, "overbrace": 1, "underbrace": 1,
	"overrightarrow": 1, "overleftarrow": 1, "overleftrightarrow": 1,
	"underrightarrow": 1, "underleftarrow": 1, "underleftrightarrow": 1,
	"overset": 2, "underset": 2, "stackrel": 2,
	"xrightarrow": 1, "xleftarrow": 1,
	"cancel": 1, "bcancel": 1, "xcancel": 1, "cancelto": 2,
	"boxed": 1, "substack": 1,

	// Math alphabets and symbols.
	"mathbb": 1, "mathbf": 1, "mathcal": 1, "mathfrak": 1, "mathrm": 1,
	"mathit": 1, "mathsf": 1, "mathtt": 1, "mathscr": 1, "mathnormal": 1,
	"mathbfit": 1, "boldsymbol": 1, "bm": 1, "pmb": 1,
	"operatorname": 1, "operatorname*": 1,
	"text": 1, "textrm": 1, "textbf": 1, "textit": 1, "textsf": 1,
	"texttt": 1, "textsc": 1, "textup": 1, "textsl": 1, "textmd": 1,
	"textnormal": 1, "emph": 1, "textsuperscript": 1, "textsubscript": 1,
	"pmod": 1, "pod": 1,
	"phantom": 1, "hphantom": 1, "vphantom": 1, "smash": 1,
	"mathclap": 1, "mathllap": 1, "mathrlap": 1, "clap": 1, "llap": 1, "rlap": 1,

	// Letter-named text accents (\c c, \v s); the symbol accents (\"o) are
	// not command names and are handled by the braced-args check directly.
	"c": 1, "v": 1, "H": 1, "u": 1, "b": 1, "d": 1, "t": 1, "r": 1, "k": 1,

	// Boxes and colour.
	"mbox": 1, "makebox": 1, "fbox": 1, "framebox": 1, "parbox": 2,
	"raisebox": 2, "scalebox": 2, "resizebox": 3, "rotatebox": 2,
	"color": 1, "textcolor": 2, "colorbox": 2, "fcolorbox": 3,

	// Sectioning and document structure.
	"part": 1, "chapter": 1, "section": 1, "subsection": 1,
	"subsubsection": 1, "paragraph": 1, "subparagraph": 1,
	"part*": 1, "chapter*": 1, "section*": 1, "subsection*": 1,
	"subsubsection*": 1, "paragraph*": 1, "subparagraph*": 1,
	"caption": 1, "footnote": 1, "footnotetext": 1,
	"title": 1, "author": 1, "date": 1, "thanks": 1,
	"begin": 1, "end": 1,
	"documentclass": 1, "usepackage": 1, "RequirePackage": 1,
	"input": 1, "include": 1, "includeonly": 1, "includegraphics": 1,
	"hspace": 1, "hspace*": 1, "vspace": 1, "vspace*": 1,

	// Labels, references, citations, links.
	"label": 1, "ref": 1, "pageref": 1, "eqref": 1, "autoref": 1,
	"nameref": 1, "cref": 1, "Cref": 1, "cpageref": 1, "Cpageref": 1,
	"crefrange": 2, "Crefrange": 2, "labelcref": 1, "vref": 1, "Vref": 1,
	"cite": 1, "citep": 1, "citet": 1, "citealp": 1, "citealt": 1,
	"citeauthor": 1, "citeyear": 1, "parencite": 1, "textcite": 1,
	"autocite": 1, "footcite": 1, "nocite": 1,
	"bibliography": 1, "bibliographystyle": 1, "addbibresource": 1,
	"url": 1, "href": 2, "hyperref": 1,
}
