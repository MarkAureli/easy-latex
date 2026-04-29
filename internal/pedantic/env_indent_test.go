package pedantic

import (
	"reflect"
	"testing"
)

func TestEnvIndent_NestedItemize(t *testing.T) {
	in := []string{
		`\begin{document}`,
		`\begin{itemize}`,
		`\item one`,
		`\begin{itemize}`,
		`\item nested`,
		`\end{itemize}`,
		`\end{itemize}`,
		`\end{document}`,
	}
	want := []string{
		`\begin{document}`,
		`\begin{itemize}`,
		`    \item one`,
		`    \begin{itemize}`,
		`        \item nested`,
		`    \end{itemize}`,
		`\end{itemize}`,
		`\end{document}`,
	}
	out, changed := fixEnvIndent("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changed=true")
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got:\n%q\nwant:\n%q", out, want)
	}
}

func TestEnvIndent_DocumentBodyFlush(t *testing.T) {
	// document is transparent → body sits at depth 0.
	in := []string{
		`\begin{document}`,
		`    Hello world`,
		`\end{document}`,
	}
	want := []string{
		`\begin{document}`,
		`Hello world`,
		`\end{document}`,
	}
	out, changed := fixEnvIndent("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changed=true")
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got:\n%q\nwant:\n%q", out, want)
	}
}

func TestEnvIndent_MathEnvIndents(t *testing.T) {
	in := []string{
		`\begin{document}`,
		`\begin{equation}`,
		`a + b = c`,
		`\end{equation}`,
		`\end{document}`,
	}
	want := []string{
		`\begin{document}`,
		`\begin{equation}`,
		`    a + b = c`,
		`\end{equation}`,
		`\end{document}`,
	}
	out, _ := fixEnvIndent("t.tex", append([]string(nil), in...))
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got:\n%q\nwant:\n%q", out, want)
	}
}

func TestEnvIndent_VerbatimUntouched(t *testing.T) {
	in := []string{
		`\begin{document}`,
		`\begin{verbatim}`,
		`   weird  spacing  here`,
		`	tab line`,
		`\end{verbatim}`,
		`\end{document}`,
	}
	out, _ := fixEnvIndent("t.tex", append([]string(nil), in...))
	// body of verbatim and \end{verbatim} preserved exactly
	if out[2] != `   weird  spacing  here` {
		t.Errorf("verbatim body line modified: %q", out[2])
	}
	if out[3] != "\ttab line" {
		t.Errorf("verbatim tab line modified: %q", out[3])
	}
	if out[4] != `\end{verbatim}` {
		t.Errorf("\\end{verbatim} modified: %q", out[4])
	}
}

func TestEnvIndent_DisplayMathBrackets(t *testing.T) {
	in := []string{
		`\begin{document}`,
		`\[`,
		`a = b`,
		`\]`,
		`\end{document}`,
	}
	want := []string{
		`\begin{document}`,
		`\[`,
		`    a = b`,
		`\]`,
		`\end{document}`,
	}
	out, _ := fixEnvIndent("t.tex", append([]string(nil), in...))
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got:\n%q\nwant:\n%q", out, want)
	}
}

func TestEnvIndent_TabsInLeadingWSNormalized(t *testing.T) {
	in := []string{
		`\begin{document}`,
		`\begin{itemize}`,
		"\t\\item tabbed",
		`\end{itemize}`,
		`\end{document}`,
	}
	want := []string{
		`\begin{document}`,
		`\begin{itemize}`,
		`    \item tabbed`,
		`\end{itemize}`,
		`\end{document}`,
	}
	out, changed := fixEnvIndent("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changed=true")
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got:\n%q\nwant:\n%q", out, want)
	}
}

func TestEnvIndent_CommentOnlyLineIndented(t *testing.T) {
	in := []string{
		`\begin{document}`,
		`\begin{itemize}`,
		`% a comment`,
		`\item one`,
		`\end{itemize}`,
		`\end{document}`,
	}
	want := []string{
		`\begin{document}`,
		`\begin{itemize}`,
		`    % a comment`,
		`    \item one`,
		`\end{itemize}`,
		`\end{document}`,
	}
	out, _ := fixEnvIndent("t.tex", append([]string(nil), in...))
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got:\n%q\nwant:\n%q", out, want)
	}
}

func TestEnvIndent_BlankLinesUntouched(t *testing.T) {
	in := []string{
		`\begin{document}`,
		`\begin{itemize}`,
		``,
		`\item one`,
		`   `, // ws-only line
		`\end{itemize}`,
		`\end{document}`,
	}
	out, _ := fixEnvIndent("t.tex", append([]string(nil), in...))
	if out[2] != `` {
		t.Errorf("blank line modified: %q", out[2])
	}
	if out[4] != `   ` {
		t.Errorf("ws-only line modified: %q", out[4])
	}
}

func TestEnvIndent_Detector(t *testing.T) {
	lines := []string{
		`\begin{document}`,           // line 1: depth 0, ok
		`\begin{itemize}`,            // line 2: depth 0, ok
		`\item one`,                  // line 3: should be 4 spaces, got 0 → flag
		`    \item two`,              // line 4: ok
		`        \item over-indented`, // line 5: should be 4, got 8 → flag
		`\end{itemize}`,              // line 6: depth 0, ok
		`\end{document}`,             // line 7: ok
	}
	diags := checkEnvIndent("t.tex", lines)
	wantLines := []int{3, 5}
	if len(diags) != len(wantLines) {
		t.Fatalf("got %d diags, want %d: %+v", len(diags), len(wantLines), diags)
	}
	for i, want := range wantLines {
		if diags[i].Line != want {
			t.Errorf("diag[%d].Line = %d, want %d (msg=%q)", i, diags[i].Line, want, diags[i].Message)
		}
	}
}

func TestEnvIndent_MultilineBraceArg(t *testing.T) {
	in := []string{
		`\usetikzlibrary{`,
		`    shapes.geometric,`,
		`    arrows`,
		`}`,
	}
	out, changed := fixEnvIndent("t.tex", append([]string(nil), in...))
	if changed {
		t.Errorf("expected no change, got:\n%q", out)
	}
	if diags := checkEnvIndent("t.tex", in); len(diags) != 0 {
		t.Errorf("expected no diags, got: %+v", diags)
	}
}

func TestEnvIndent_MultilineBraceArgWrongIndent(t *testing.T) {
	in := []string{
		`\hypersetup{`,
		`colorlinks = true,`,
		`}`,
	}
	diags := checkEnvIndent("t.tex", in)
	if len(diags) != 1 || diags[0].Line != 2 {
		t.Errorf("expected 1 diag on line 2, got: %+v", diags)
	}
	want := []string{
		`\hypersetup{`,
		`    colorlinks = true,`,
		`}`,
	}
	out, changed := fixEnvIndent("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changed=true")
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got:\n%q\nwant:\n%q", out, want)
	}
}

func TestEnvIndent_NewcommandWithBracketArg(t *testing.T) {
	// \newcommand{name}[n]{ ... } leaves one brace open at end of first line.
	in := []string{
		`\makeatletter`,
		`\newcommand{\foo}[3]{%`,
		`    \edef#2{\the\pgf@x}%`,
		`}`,
		`\makeatother`,
	}
	if diags := checkEnvIndent("t.tex", in); len(diags) != 0 {
		t.Errorf("expected no diags, got: %+v", diags)
	}
}

func TestEnvIndent_NestedBraces(t *testing.T) {
	in := []string{
		`\foo{`,
		`    \bar{`,
		`        baz`,
		`    }`,
		`}`,
	}
	if diags := checkEnvIndent("t.tex", in); len(diags) != 0 {
		t.Errorf("expected no diags, got: %+v", diags)
	}
}

func TestEnvIndent_BraceInsideEnv(t *testing.T) {
	// Brace depth stacks on top of env depth.
	in := []string{
		`\begin{document}`,
		`\begin{itemize}`,
		`    \item \foo{`,
		`        bar`,
		`    }`,
		`\end{itemize}`,
		`\end{document}`,
	}
	if diags := checkEnvIndent("t.tex", in); len(diags) != 0 {
		t.Errorf("expected no diags, got: %+v", diags)
	}
}

func TestEnvIndent_EscapedBracesIgnored(t *testing.T) {
	// \{ and \} should not affect brace depth.
	in := []string{
		`a \{ b \} c`,
		`d`,
	}
	if diags := checkEnvIndent("t.tex", in); len(diags) != 0 {
		t.Errorf("expected no diags, got: %+v", diags)
	}
}

func TestEnvIndent_SquareBracketsNotTracked(t *testing.T) {
	// Math interval notation `[0, \infty)` is unmatched at the source level.
	// Square brackets must not push brace depth or following lines cascade.
	in := []string{
		`For all $\lambda \in [0, \infty)$ it holds that`,
		`stuff happens.`,
		`More stuff.`,
	}
	if diags := checkEnvIndent("t.tex", in); len(diags) != 0 {
		t.Errorf("expected no diags, got: %+v", diags)
	}
}

func TestEnvIndent_CommentedBraceIgnored(t *testing.T) {
	// A `{` inside a comment must not push brace depth.
	in := []string{
		`foo % opens here {`,
		`bar`,
	}
	if diags := checkEnvIndent("t.tex", in); len(diags) != 0 {
		t.Errorf("expected no diags, got: %+v", diags)
	}
}

func TestEnvIndent_BraceWrappedEndBody(t *testing.T) {
	// \newenvironment second-arg body `{\end{env}}` must register as env-end
	// so subsequent lines aren't shifted by phantom env depth.
	in := []string{
		`\newenvironment{wrap}[1]{`,
		`    \begin{subequations}`,
		`        body`,
		`    }`,
		`{\end{subequations}}`,
		`after`,
	}
	if diags := checkEnvIndent("t.tex", in); len(diags) != 0 {
		t.Errorf("expected no diags, got: %+v", diags)
	}
}

func TestEnvIndent_InlineCasesInsideAlign(t *testing.T) {
	// `\begin{cases}` mid-line in math is idiomatic. Body must be tracked at
	// align+cases depth (8 spaces), and \end{cases} must close cases — not
	// silently pop align.
	in := []string{
		`\begin{align}\label{eq:foo}`,
		`    \tilde{c}(\bm{b}) \coloneqq \begin{cases}`,
		`        c(\bm{b}),& \text{if } x`,
		`        c(\bm{b}) + \alpha,& \text{else}.`,
		`    \end{cases}`,
		`\end{align}`,
		`after`,
	}
	if diags := checkEnvIndent("t.tex", in); len(diags) != 0 {
		t.Errorf("expected no diags, got: %+v", diags)
	}
}

func TestEnvIndent_TwoBeginsOnOneLine(t *testing.T) {
	in := []string{
		`\begin{outer}\begin{inner}`,
		`        body`,
		`\end{inner}\end{outer}`,
	}
	if diags := checkEnvIndent("t.tex", in); len(diags) != 0 {
		t.Errorf("expected no diags, got: %+v", diags)
	}
}

func TestEnvIndent_PreambleAtDepthZero(t *testing.T) {
	in := []string{
		`\documentclass{article}`,
		`\usepackage{amsmath}`,
		`\begin{document}`,
		`Body.`,
		`\end{document}`,
	}
	out, changed := fixEnvIndent("t.tex", append([]string(nil), in...))
	if changed {
		t.Errorf("expected no change, got:\n%q", out)
	}
}
