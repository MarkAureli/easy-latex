package pedantic

import (
	"strings"
	"testing"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// bareWords returns the bare-word diagnostics of math-token-spacing.
func bareWords(line string) []Diagnostic {
	var out []Diagnostic
	for _, d := range checkMathTokenSpacing(nil, "test.tex", []string{line}) {
		if strings.HasPrefix(d.Message, "bare word") {
			out = append(out, d)
		}
	}
	return out
}

func TestMathBareWord(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		count int
	}{
		// single letters are fine
		{"single letter inline", `$x = 5$`, 0},
		{"single letter subscript", `$x_i = 5$`, 0},
		// commands are fine
		{"command", `$\pauli = 5$`, 0},
		{"command with arg", `$\alpha + \beta = 1$`, 0},
		// \text family exempt
		{"text", `$\text{Pauli} = 5$`, 0},
		{"textbf", `$\textbf{abc} = 5$`, 0},
		{"textrm", `$\textrm{abc} = 5$`, 0},
		{"textit", `$\textit{abc} = 5$`, 0},
		{"textsf", `$\textsf{abc} = 5$`, 0},
		{"texttt", `$\texttt{abc} = 5$`, 0},
		// math font commands exempt
		{"mathrm", `$\mathrm{abc} = 5$`, 0},
		{"mathsf", `$\mathsf{abc} = 5$`, 0},
		{"mathtt", `$\mathtt{abc} = 5$`, 0},
		{"mathit", `$\mathit{abc} = 5$`, 0},
		// explicit text boxes exempt
		{"mbox", `$\mbox{text} = 5$`, 0},
		{"hbox", `$\hbox{text} = 5$`, 0},
		// bare words flagged
		{"bare word inline", `$pauli = 5$`, 1},
		{"bare word two letters", `$ab = 5$`, 1},
		{"bare word in display", `\[pauli = 5\]`, 1},
		// bare word in braces (subscript etc.) flagged
		{"bare word in braces", `$e^{ij}$`, 1},
		// mixed: text-wrapped and bare on same line
		{"text ok bare flagged", `$\text{Pauli} + pauli = 5$`, 1},
		// text outside math not flagged
		{"text outside math", `Let pauli be defined as follows.`, 0},
		// no bare word in math with numbers breaking runs
		{"letter-number-letter", `$x1y = 5$`, 0},
		// display math env
		{"display math env", `\begin{equation}pauli = 5\end{equation}`, 1},
		// nested braces inside text cmd
		{"text nested braces", `$\text{a{b}c} = 5$`, 0},
		// intertext exempt
		{"intertext", `\intertext{some text here}`, 0},
		// label argument exempt
		{"label", `$x = 5 \label{eq:pauli}$`, 0},
		{"label bare name", `$x = 5 \label{myeq}$`, 0},
		// begin/end env names exempt
		{"begin env name", `\begin{equation}x = 5\end{equation}`, 0},
		{"begin cases inside math", `\begin{align}\begin{cases}x\end{cases}\end{align}`, 0},
		// begin/end with column spec args exempt (array, tabular)
		{"begin array col spec", `\begin{array}{rrr}x\end{array}`, 0},
		{"begin array centered", `\begin{array}{ccc}x\end{array}`, 0},
		{"begin tabular col spec", `\begin{tabular}{lll}x\end{tabular}`, 0},
		{"begin array opt and arg", `\begin{array}[t]{rrr}x\end{array}`, 0},
		// hspace/vspace length args exempt
		{"hspace cm", `$x \hspace{2cm} y$`, 0},
		{"hspace em", `$x \hspace{1em} y$`, 0},
		{"hspace star", `$x \hspace*{2cm} y$`, 0},
		{"vspace cm", `$x \vspace{2cm} y$`, 0},
		{"vspace star", `$x \vspace*{1ex} y$`, 0},
		// TikZ calc expressions ($(a1.south west)-(0.1,0.1)$) are code, not math
		{"tikz anchor single word", `\begin{tikzpicture}\draw ($(a1.south)-(0.1,0.1)$);\end{tikzpicture}`, 0},
		{"tikz anchor two words", `\tikz{\draw ($(a1.south west)-(0.1,0.1)$);}`, 0},
		{"tikz anchor two coords", `\begin{tikzpicture}\draw ($(a1.south west)-(a3.south east)$);\end{tikzpicture}`, 0},
		// outside TikZ, dotted words in math are bare words
		{"bare words around dot", `$foo.south$`, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := bareWords(tt.line)
			if len(diags) != tt.count {
				t.Errorf("got %d diagnostics, want %d\nline: %s\ndiags: %v",
					len(diags), tt.count, tt.line, diags)
			}
		})
	}
}

func spaced(t *testing.T, ctx *Context, in string) string {
	t.Helper()
	out, _ := fixMathTokenSpacing(ctx, "t.tex", strings.Split(in, "\n"))
	return strings.Join(out, "\n")
}

func TestFixMathTokenSpacing(t *testing.T) {
	for in, want := range map[string]string{
		// Operators and relations.
		`$a+b=c$`:          `$a + b = c$`,
		`$x\le y$`:         `$x \le y$`,
		`$a\cdot b$`:       `$a \cdot b$`,
		`$x\in A\to B$`:    `$x \in A \to B$`,
		`$x:=y$`:           `$x := y$`,
		`$f\colon A\to B$`: `$f\colon A \to B$`,
		`$a\neq b$`:        `$a \neq b$`,
		`$a\not=b$`:        `$a \not= b$`,
		`$a\not\in b$`:     `$a \not\in b$`,
		// Punctuation.
		`$f(x,y)$`:    `$f(x, y)$`,
		`$1,\dots,n$`: `$1, \dots, n$`,
		`$n!m$`:       `$n! m$`,
		`$1.5x$`:      `$1.5 x$`,
		`$1{,}5$`:     `$1{,}5$`,
		// Unary signs.
		`$=-x$`:       `$= -x$`,
		`$a=-x$`:      `$a = -x$`,
		`$(-1)^{-1}$`: `$(-1)^{-1}$`,
		`$-x+y$`:      `$-x + y$`,
		`$a\le-1$`:    `$a \le -1$`,
		`$a,-1$`:      `$a, -1$`,
		`$a-(b)$`:     `$a - (b)$`,
		`$\pm1$`:      `$\pm1$`,
		// Juxtaposition.
		`$2x$`:           `$2 x$`,
		`$x2$`:           `$x 2$`,
		`$\alpha\beta$`:  `$\alpha \beta$`,
		`$\alpha2$`:      `$\alpha 2$`,
		`$(a)x$`:         `$(a) x$`,
		`$\frac{a}{b}c$`: `$\frac{a}{b} c$`,
		`$x^{2}y$`:       `$x^{2} y$`,
		`$x^2y$`:         `$x^2 y$`,
		`$f'x$`:          `$f' x$`,
		`$\mathrm{d}x$`:  `$\mathrm{d} x$`,
		`$\text{if}x$`:   `$\text{if} x$`,
		// Opening delimiters: function application stays tight.
		`$f(x)$`:               `$f(x)$`,
		`$f'(x)$`:              `$f'(x)$`,
		`$f^{-1}(x)$`:          `$f^{-1}(x)$`,
		`$\sin^{2}(x)$`:        `$\sin^{2}(x)$`,
		`$\phi(x)$`:            `$\phi(x)$`,
		`$\mathbf{f}(x)$`:      `$\mathbf{f}(x)$`,
		`$f\left(x\right)$`:    `$f\left(x\right)$`,
		`$2(x+1)$`:             `$2 (x + 1)$`,
		`$(a+b)(c+d)$`:         `$(a + b) (c + d)$`,
		`$x\le(a)$`:            `$x \le (a)$`,
		`$a=\left(b\right)$`:   `$a = \left(b\right)$`,
		`$\{x\in A\mid x>0\}$`: `$\{x \in A \mid x > 0\}$`,
		`$\langle x,y\rangle$`: `$\langle x, y\rangle$`,
		`$\bigl(x\bigr)y$`:     `$\bigl(x\bigr) y$`,
		// Arguments stay attached.
		`$\frac{a+b}{c}$`:              `$\frac{a + b}{c}$`,
		`$\sqrt[3]{x}$`:                `$\sqrt[3]{x}$`,
		`$\sum\limits_{i=1}^n$`:        `$\sum\limits_{i = 1}^n$`,
		`$\operatorname*{arg\,max}_x$`: `$\operatorname*{arg\,max}_x$`,
		// Left alone.
		`$|x|$`:           `$|x|$`,
		`$\|x\|$`:         `$\|x\|$`,
		`$a/b$`:           `$a/b$`,
		`$a\,b$`:          `$a\,b$`,
		`$a\quad b$`:      `$a\quad b$`,
		`$a~b$`:           `$a~b$`,
		`$xy$`:            `$xy$`,
		`$\label{a-b}$`:   `$\label{a-b}$`,
		`$\text{a+b}$`:    `$\text{a+b}$`,
		`$\hspace{1em}x$`: `$\hspace{1em}x$`,
		`a+b`:             `a+b`,
		`$a$$b$`:          `$a$$b$`,
		`$a+b$ % c+d`:     `$a + b$ % c+d`,
		`\(a+b\)`:         `\(a + b\)`,
		"\\begin{align}\na&=b+c\\\\\n&=-d\n\\end{align}": "\\begin{align}\na&= b + c\\\\\n&= -d\n\\end{align}",
		"\\begin{pmatrix}a&b\\end{pmatrix}":              "\\begin{pmatrix}a&b\\end{pmatrix}",
		"$a\n-b$":                                        "$a\n- b$", // binary: a continued line
		`$\{x\}y$`:                                       `$\{x\} y$`,
		`${x}y$`:                                         `${x}y$`,
		"$a+\n-b$":                                       "$a +\n-b$",
		// Unicode.
		`$a≤b$`: `$a ≤ b$`,
	} {
		if got := spaced(t, nil, in); got != want {
			t.Errorf("%s\n got: %s\nwant: %s", in, got, want)
		}
	}
}

func TestMathTokenSpacingProbedClasses(t *testing.T) {
	ctx := &Context{MathClasses: map[string]texscan.MathClass{"foo": texscan.MathRel, "lb": texscan.MathOpen}}
	if got := spaced(t, ctx, `$a\foo b\foo-1$`); got != `$a \foo b \foo -1$` {
		t.Errorf("got %s", got)
	}
	if got := spaced(t, ctx, `$2\lb x$`); got != `$2 \lb x$` {
		t.Errorf("got %s", got)
	}
}

func TestMathTokenSpacingDiagnostics(t *testing.T) {
	diags := checkMathTokenSpacing(nil, "t.tex", []string{`$a+b$ $pauli$`})
	var msgs []string
	for _, d := range diags {
		msgs = append(msgs, d.Message)
	}
	want := []string{"missing space in math at column 3", "missing space in math at column 4", `bare word "pauli" in math mode`}
	if strings.Join(msgs, "|") != strings.Join(want, "|") {
		t.Errorf("got %q", msgs)
	}
}

func TestFixMathTokenSpacingIdempotent(t *testing.T) {
	in := `$\frac{a+b}{2c}\le\sum_{i=1}^{n}x_i^2(1-x)\,dx,\quad f'(x)=-\alpha\beta$`
	once := spaced(t, nil, in)
	if twice := spaced(t, nil, once); twice != once {
		t.Errorf("not idempotent:\n%s\n%s", once, twice)
	}
}
