package texscan

import (
	"strings"
	"testing"
)

// classLetters renders classes as one letter per byte:
// T text, S standalone, H heading, C cell, M math, k markup, o opaque, v verbatim.
func classLetters(cs []Class) string {
	const letters = "TSHCMkov"
	var b strings.Builder
	for _, c := range cs {
		b.WriteByte(letters[c])
	}
	return b.String()
}

// proseOf returns line with every non-prose byte replaced by '_'.
func proseOf(line string, cs []Class) string {
	b := []byte(line)
	for i, c := range cs {
		if !c.IsProse() {
			b[i] = '_'
		}
	}
	return string(b)
}

// classOf returns, per line, the bytes of class want with others as '_'.
func classOf(lines []string, want Class) []string {
	out := make([]string, len(lines))
	for li, cs := range Classify(lines) {
		b := []byte(lines[li])
		for i, c := range cs {
			if c != want {
				b[i] = '_'
			}
		}
		out[li] = string(b)
	}
	return out
}

func TestClassify_Letters(t *testing.T) {
	tests := []struct{ in, want string }{
		{`a \emph{b} $x$`, `TTkkkkkkTkTkMk`},
		{`\section[s]{T} x`, `kkkkkkkkkHkkHkTT`},
		{`\label{eq} \footnote{n}`, `kkkkkkkookTkkkkkkkkkkSk`},
		{`\verb|%x| y`, `kkkkkkvvkTT`},
	}
	for _, tc := range tests {
		got := classLetters(Classify([]string{tc.in})[0])
		if got != tc.want {
			t.Errorf("%s\n got  %s\n want %s", tc.in, got, tc.want)
		}
	}
}

func TestClassify_Prose(t *testing.T) {
	tests := []struct{ in, want string }{
		// Unknown and project-defined commands: all arguments opaque.
		{`see \mycmd[o]{a}{b} and`, `see _______________ and`},
		{`\cite[p.~5]{key} text`, `________________ text`},
		// Known commands: per-argument roles.
		{`\textcolor{red}{word} x`, `________________word_ x`},
		{`\href{http://a-b.c}{link}`, `____________________link_`},
		{`\multicolumn{2}{c}{cell}`, `___________________cell_`},
		{`\caption[short]{long}`, `_________short__long_`},
		{`\item[Term] text`, `______Term_ text`},
		// Plain groups inherit the context.
		{`{\bfseries bold} x`, `__________ bold_ x`},
		// Math and its prose islands.
		{`$a \text{if} b$ x`, `_______________ x`},
		{`\begin{align}a \intertext{so} b\end{align}`, `__________________________so______________`},
		{`$\mathrm{d}x \label{e}$`, `_______________________`},
		// Definitions are opaque.
		{`\newcommand{\foo}[1]{body #1} x`, `_____________________________ x`},
		{`\def\foo#1{body} x`, `________________ x`},
		// \\[len] and escapes.
		{`a\\[2pt] b \% c`, `a_______ b __ c`},
		// Accents keep the accented letter.
		{`Ren\'e`, `Ren_'e`},
	}
	for _, tc := range tests {
		got := proseOf(tc.in, Classify([]string{tc.in})[0])
		if got != tc.want {
			t.Errorf("%s\n got  %q\n want %q", tc.in, got, tc.want)
		}
	}
}

func TestClassify_Environments(t *testing.T) {
	src := []string{
		`\begin{tabular}{l|c}`,
		`a & b \\`,
		`\end{tabular}`,
		`\begin{equation}`,
		`x = \text{yes}`,
		`\end{equation}`,
		`\begin{verbatim}`,
		`{ $ \x`,
		`\end{verbatim}`,
		`\begin{theorem}[Name] body`,
		`\end{theorem}`,
	}
	cs := Classify(src)
	want := []string{
		`kkkkkkkkkkkkkkkkoook`, // column spec opaque
		`CCkCCCkk`,
		`kkkkkkkkkkkkk`,
		`kkkkkkkkkkkkkkkk`,
		`MMMMkkkkkkoook`, // \text inside math is opaque
		`kkkkkkkkkkkkkk`,
		`kkkkkkkkkkkkkkkk`,
		`vvvvvv`,
		`kkkkkkkkkkkkkk`,
		`kkkkkkkkkkkkkkkkooookTTTTT`, // env options opaque
		`kkkkkkkkkkkkk`,
	}
	for li := range src {
		if got := classLetters(cs[li]); got != want[li] {
			t.Errorf("line %d %q\n got  %s\n want %s", li, src[li], got, want[li])
		}
	}
}

func TestClassify_MultiLineArgs(t *testing.T) {
	src := []string{
		`\begin{tcolorbox}[title=A,`,
		`  colback=red]`,
		`Body text`,
		`\end{tcolorbox}`,
		`\mycmd{first`,
		`second} after`,
	}
	got := classOf(src, ClassText)
	want := []string{
		`__________________________`,
		`______________`,
		`Body text`,
		`_______________`,
		`____________`,
		`_______ after`,
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d\n got  %q\n want %q", i, got[i], want[i])
		}
	}
}

func TestClassify_LineBreakEndsUnknownArgs(t *testing.T) {
	src := []string{`\vfill`, `{\scriptsize Text}`, `\addtolength{\parskip}{1pt}`, `  {\large Name}`, `\section`, `{Title}`}
	got := classOf(src, ClassText)
	if got[1] != `____________ Text_` || got[3] != `  _______ Name_` {
		t.Errorf("group after unknown command on next line: %q", got)
	}
	if h := classOf(src, ClassHeading); h[5] != `_Title_` {
		t.Errorf("mandatory arg on next line: %q", h[5])
	}
}

func TestClassify_ParagraphBreakEndsArgs(t *testing.T) {
	src := []string{`\mycmd`, ``, `{group} text`}
	got := classOf(src, ClassText)
	if got[2] != `_group_ text` {
		t.Errorf("got %q", got[2])
	}
}

func TestClassify_TikZ(t *testing.T) {
	src := []string{
		`\begin{tikzpicture}[every node/.style={draw}]`,
		`  \draw[->] (0,0) -- (1,1) node[above] {Edge label};`,
		`  \node (x)   at (-1,0) {pages 1-5 $x$};`,
		`  \foreach \x in {1,...,3} \draw (\x,0) -- (\x,1);`,
		`  \draw (a) edge node {on edge} (b);`,
		`\end{tikzpicture}`,
	}
	got := classOf(src, ClassHeading)
	want := []string{
		`_____________________________________________`,
		`________________________________________Edge label__`,
		`_________________________pages 1-5 _____`,
		`__________________________________________________`,
		`_______________________on edge______`,
		`_________________`,
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d\n got  %q\n want %q", i, got[i], want[i])
		}
	}
	// Path syntax and spacing are opaque, not prose.
	for li, cs := range Classify(src) {
		for i, c := range cs {
			if c == ClassText {
				t.Errorf("line %d col %d: running text in TikZ code: %q", li, i, src[li])
			}
		}
	}
}

func TestClassify_PgfplotsProseKeys(t *testing.T) {
	src := []string{
		`\begin{axis}[xlabel={sample probability}, ylabel=Time in s,`,
		`  xmin=0, title={With $n$ steps}]`,
		`\addplot[blue] table {data.dat};`,
		`\addlegendentry{Simulation}`,
		`\end{axis}`,
	}
	got := classOf(src, ClassHeading)
	want := []string{
		`_____________________sample probability__________Time in s_`,
		`_________________With ___ steps__`,
		`________________________________`,
		`________________Simulation_`,
		`__________`,
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d\n got  %q\n want %q", i, got[i], want[i])
		}
	}
}

func TestClassify_Quantikz(t *testing.T) {
	src := []string{`\begin{quantikz}`, `\lstick{$\ket{0}$} & \gate{H} & \qw`, `\end{quantikz}`}
	for li, cs := range Classify(src) {
		for i, c := range cs {
			if c.IsProse() {
				t.Errorf("line %d col %d: prose in quantikz: %q", li, i, src[li])
			}
		}
	}
}

func TestClassify_UnbalancedRecovers(t *testing.T) {
	src := []string{`\emph{a $x} b`, `after`}
	cs := Classify(src)
	if got := proseOf(src[1], cs[1]); got != "after" {
		t.Errorf("after unbalanced math: %q", got)
	}
}
