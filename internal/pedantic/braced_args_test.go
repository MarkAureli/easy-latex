package pedantic

import (
	"maps"
	"math/rand"
	"strings"
	"testing"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func bracedCtx() *Context {
	args := maps.Clone(texscan.BuiltinCmdArgs)
	maps.Copy(args, map[string]int{"alpha": 0, "beta": 0, "prime": 0, "R": 0, "xfoo": 2, "xfoo*": 2, "lvert": 0})
	return &Context{CmdArgs: args}
}

func TestBracedArgs_Fix(t *testing.T) {
	cases := []struct{ in, want string }{
		// Scripts.
		{`$x^2 + y_i$`, `$x^{2} + y_{i}$`},
		{`$x^{2} + y_{ij}$`, `$x^{2} + y_{ij}$`},
		{`$x^10$`, `$x^{1}0$`},
		{`$x^ a$`, `$x^{a}$`},
		{`$x^\alpha_\beta$`, `$x^{\alpha}_{\beta}$`},
		{`$x^*$ $x^-$`, `$x^{*}$ $x^{-}$`},
		{`$x_\text{max}$`, `$x_{\text{max}}$`},
		{`$x^\mathbf a$`, `$x^{\mathbf{a}}$`},
		{`$x^\frac12$`, `$x^{\frac{1}{2}}$`},
		{`\[ \sum_i^n x \]`, `\[ \sum_{i}^{n} x \]`},
		{`$x^ä$`, `$x^{ä}$`},
		// Macro arguments.
		{`$\frac12 + \frac 1 2 + \frac{1}2$`, `$\frac{1}{2} + \frac{1}{2} + \frac{1}{2}$`},
		{`$\frac\alpha\beta$`, `$\frac{\alpha}{\beta}$`},
		{`$\mathbb R \hat x \sqrt2 \sqrt[3]x$`, `$\mathbb{R} \hat{x} \sqrt{2} \sqrt[3]{x}$`},
		{`$\frac{\mathbb R}2$`, `$\frac{\mathbb{R}}{2}$`},
		{`$x^\frac{\mathbb R}2$`, `$x^{\frac{\mathbb{R}}{2}}$`}, // second pass
		{`Text \textbf x and \emph{y}.`, `Text \textbf{x} and \emph{y}.`},
		{`\section*A`, `\section*{A}`},
		{`$\xfoo* a[b] c$`, `$\xfoo*{a}[b]{c}$`},
		{`$x$ % \frac12 in a comment`, `$x$ % \frac12 in a comment`},
		// Text accents.
		{`Schr\"odinger, G\" odel, Erd\H os`, `Schr\"{o}dinger, G\"{o}del, Erd\H{o}s`},
		{"\\'e \\`a \\^o \\~n \\=a \\.z \\c c \\v s \\u g \\k a \\r a", "\\'{e} \\`{a} \\^{o} \\~{n} \\={a} \\.{z} \\c{c} \\v{s} \\u{g} \\k{a} \\r{a}"},
		{`Ni\'\i ez \'ä`, `Ni\'{\i}ez \'{ä}`},
		{`\textbf\ae x and \textbf\" o x`, `\textbf{\ae}x and \textbf{\"{o}} x`},
		{`$x^\alpha b$`, `$x^{\alpha}b$`},
		{`\textbf\"o and \emph{\'e}`, `\textbf{\"{o}} and \emph{\'{e}}`},
		{`\section{Schr\"odinger}`, `\section{Schr\"{o}dinger}`},
		{`$\text{G\"odel}$`, `$\text{G\"{o}del}$`},
	}
	for _, c := range cases {
		got, _ := fixBracedArgs(bracedCtx(), "t.tex", []string{c.in})
		if got[0] != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.in, got[0], c.want)
		}
	}
}

func TestBracedArgs_LeftAlone(t *testing.T) {
	for _, in := range []string{
		`\url|a_b| \verb|x^2| \label{a_b} \includegraphics{fig_1}`,
		`$\not= \left( x \right) \big( a^{\left(} $`,
		`$a^^M$ \$x^2`,
		`G\"{o}del, Ma\~{n}ana, \'{\i}, \c{c}`, // already braced
		`$\d x \b v \v s$`,                     // letter accents only in prose
		"\\begin{tabbing} A \\= B \\' C \\` D", // tabbing: \= \' \` are tab commands
		`\"[x] \'$x$`,                          // no options, punctuation or math``
		`$x^\unknown a$`,                       // unknown command might take more tokens
		`$\frac{1}$`,                           // second argument on the next line
		`\textbf|x|`,                           // punctuation is never a macro argument
		`\newcommand\R{\mathbb R}`,             // definition bodies are opaque
		`\def\foo#1{\hat#1}`,                   //
		`$\text{a_b x^2}$`,                     // prose inside math
		`$x^{\frac12}$ is fixed, $y$`,          // (nested: only \frac)
	} {
		lines := []string{in}
		diags := checkBracedArgs(bracedCtx(), "t.tex", lines)
		if in == `$x^{\frac12}$ is fixed, $y$` {
			if len(diags) != 2 {
				t.Errorf("%s: want 2 diags, got %v", in, diags)
			}
			continue
		}
		if len(diags) != 0 {
			t.Errorf("%s: unexpected %v", in, diags)
		}
	}
}

func TestBracedArgs_Diagnostics(t *testing.T) {
	diags := checkBracedArgs(bracedCtx(), "t.tex", []string{`$x^2 \frac 1{2}$`})
	var msgs []string
	for _, d := range diags {
		msgs = append(msgs, d.Message)
	}
	want := "unbraced superscript at column 4|unbraced argument of \\frac at column 12"
	if strings.Join(msgs, "|") != want {
		t.Errorf("got %q", strings.Join(msgs, "|"))
	}
	if d := checkBracedArgs(nil, "t.tex", []string{`$\frac12$`}); len(d) != 0 {
		t.Errorf("nil context knows no commands: %v", d)
	}
}

func TestBracedArgs_Idempotent(t *testing.T) {
	in := []string{`$x^\frac\alpha 2_\mathbb R \sqrt[3]\hat x$`}
	once, _ := fixBracedArgs(bracedCtx(), "t.tex", append([]string(nil), in...))
	twice, changed := fixBracedArgs(bracedCtx(), "t.tex", append([]string(nil), once...))
	if changed || twice[0] != once[0] {
		t.Errorf("not idempotent: %s -> %s", once[0], twice[0])
	}
	if len(checkBracedArgs(bracedCtx(), "t.tex", once)) != 0 {
		t.Errorf("diagnostics remain after fix: %s", once[0])
	}
}

func TestBracedArgs_RandomIdempotent(t *testing.T) {
	toks := []string{`$`, `\[`, `\]`, `^`, `_`, `{`, `}`, `[`, `]`, ` `, `a`, `1`, `+`, `*`, `ä`, `\frac`, `\mathbb`, `\sqrt`, `\hat`, `\alpha`, `\text`, `\textbf`, `\section*`, `\xfoo*`, `\left(`, `\right)`, `\not`, `\unknown`, `\,`, `\\`, `%`, `\url|`, `|`, `\"`, "\\'", `\i`, `\c`, `\begin{tabbing}`, `\end{tabbing}`, `\textbf`, `x`}
	rng := rand.New(rand.NewSource(7))
	for it := 0; it < 20000; it++ {
		var b strings.Builder
		for k := 1 + rng.Intn(12); k > 0; k-- {
			b.WriteString(toks[rng.Intn(len(toks))])
		}
		in := b.String()
		once, _ := fixBracedArgs(bracedCtx(), "t.tex", []string{in})
		twice, changed := fixBracedArgs(bracedCtx(), "t.tex", []string{once[0]})
		if changed {
			t.Fatalf("not idempotent:\n in    %q\n once  %q\n twice %q", in, once[0], twice[0])
		}
		if d := checkBracedArgs(bracedCtx(), "t.tex", once); len(d) != 0 {
			t.Fatalf("diagnostics after fix: %q -> %q: %v", in, once[0], d)
		}
		if strings.Count(once[0], "{")-strings.Count(once[0], "}") != strings.Count(in, "{")-strings.Count(in, "}") {
			t.Fatalf("brace balance changed: %q -> %q", in, once[0])
		}
	}
}
