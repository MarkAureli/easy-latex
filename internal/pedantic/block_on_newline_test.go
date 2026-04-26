package pedantic

import (
	"reflect"
	"testing"
)

func TestBlockOnNewline_Detector(t *testing.T) {
	lines := []string{
		`\section{Intro} \subsection{Bg}`, // line 1: subsection mid-line → flag (leading)
		`Hello \\ world`,                  // line 2: \\ has content after → flag (trailing)
		`  \section{Foo}`,                 // line 3: leading-ws ok
		`\begin{itemize} \item one`,       // line 4: \item mid-line → flag (leading)
		`text \input{foo} more`,           // line 5: \input mid-line → flag (leading)
		`text \label{x} ok`,               // line 6: \label not block → no flag
		`text \hspace*{1em} ok`,           // line 7: \hspace* not block → no flag
		`Some text\\`,                     // line 8: \\ at end of line → no flag
		`Para break\newline`,              // line 9: \newline at end of line → no flag
	}
	diags := checkBlockOnNewline("t.tex", lines)
	wantLines := []int{1, 2, 4, 5}
	if len(diags) != len(wantLines) {
		t.Fatalf("got %d diags, want %d: %+v", len(diags), len(wantLines), diags)
	}
	for i, want := range wantLines {
		if diags[i].Line != want {
			t.Errorf("diag[%d].Line = %d, want %d (msg=%q)", i, diags[i].Line, want, diags[i].Message)
		}
	}
}

func TestBlockOnNewline_MathSkipped(t *testing.T) {
	// `\\` inside math envs is a row separator and must not be flagged.
	// Nested `\begin{cases}` mid-equation is also legitimate.
	lines := []string{
		`\begin{align}`,
		`    f(x) \coloneqq \begin{cases} a \\ b \end{cases} \\`,
		`    g(x) \coloneqq c. \\`,
		`\end{align}`,
	}
	diags := checkBlockOnNewline("t.tex", lines)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags inside math, got %d: %+v", len(diags), diags)
	}
}

func TestBlockOnNewline_VerbatimSkipped(t *testing.T) {
	lines := []string{
		`\begin{verbatim}`,
		`text \section{X} more`, // would normally flag, but inside verbatim
		`\end{verbatim}`,
	}
	diags := checkBlockOnNewline("t.tex", lines)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags inside verbatim, got %d: %+v", len(diags), diags)
	}
}

func TestBlockOnNewline_Fixer(t *testing.T) {
	in := []string{
		`\section{Intro} \subsection{Bg}`,
		`  text \\ more`,
		`\begin{itemize} \item one`,
		`clean line`,
		`At end\\`, // already trailing, no change
	}
	want := []string{
		`\section{Intro}`,
		`\subsection{Bg}`,
		`  text \\`,
		`  more`,
		`\begin{itemize}`,
		`\item one`,
		`clean line`,
		`At end\\`,
	}
	out, changed := fixBlockOnNewline("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changed=true")
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", out, want)
	}
}

func TestBlockOnNewline_FixerPreservesComment(t *testing.T) {
	in := []string{
		`\section{A} \section{B} % trailing comment`,
	}
	want := []string{
		`\section{A}`,
		`\section{B} % trailing comment`,
	}
	out, changed := fixBlockOnNewline("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changed=true")
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got:  %q\nwant: %q", out, want)
	}
}

func TestBlockOnNewline_EndVerbatimMidLineFlagged(t *testing.T) {
	lines := []string{
		`\begin{verbatim}`,
		`raw text`,
		`\end{verbatim} trailing text`,
	}
	diags := checkBlockOnNewline("t.tex", lines)
	// `\end{verbatim}` is at column 0 — allowed even though "trailing text"
	// follows on the same line (the env closer is leading, not trailing).
	if len(diags) != 0 {
		t.Errorf("expected 0 diags, got %d: %+v", len(diags), diags)
	}

	lines2 := []string{
		`\begin{verbatim}`,
		`raw`,
		`text \end{verbatim}`,
	}
	diags2 := checkBlockOnNewline("t.tex", lines2)
	if len(diags2) != 1 || diags2[0].Line != 3 {
		t.Errorf("expected 1 diag on line 3, got %+v", diags2)
	}
}

func TestBlockOnNewline_NoFalsePositiveAtIndent(t *testing.T) {
	lines := []string{
		`    \item indented item content stays`,
		`\begin{document}`,
		`\end{document}`,
	}
	diags := checkBlockOnNewline("t.tex", lines)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags, got %d: %+v", len(diags), diags)
	}
}

func TestBlockOnNewline_BraceGroupAllowed(t *testing.T) {
	// Macro-definition bodies wrapping a leading block token in a brace group
	// (e.g. inside \NewDocumentEnvironment{name}{begin-body}{end-body}) must
	// not be flagged.
	lines := []string{
		` {\end{subequations}}`,
		`{\begin{itemize}}`,
		`  { \section{Foo}}`,
	}
	diags := checkBlockOnNewline("t.tex", lines)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags, got %d: %+v", len(diags), diags)
	}
}
