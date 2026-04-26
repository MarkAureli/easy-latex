package pedantic

import (
	"reflect"
	"testing"
)

func TestBlockOnNewline_Detector(t *testing.T) {
	lines := []string{
		`\section{Intro} \subsection{Bg}`, // line 1: subsection mid-line → flag
		`Hello \\ world`,                  // line 2: \\ mid-line → flag
		`  \section{Foo}`,                 // line 3: leading-ws ok
		`\begin{itemize} \item one`,       // line 4: \item mid-line → flag
		`text \input{foo} more`,           // line 5: \input mid-line → flag
		`text \label{x} ok`,               // line 6: \label not block → no flag
		`text \hspace*{1em} ok`,           // line 7: \hspace* not block → no flag
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
	}
	want := []string{
		`\section{Intro}`,
		`\subsection{Bg}`,
		`  text`,
		`  \\ more`,
		`\begin{itemize}`,
		`\item one`,
		`clean line`,
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
	// After the verbatim env closes, `\end{verbatim}` itself is a block
	// token — if it sits mid-line, it should be flagged.
	lines := []string{
		`\begin{verbatim}`,
		`raw text`,
		`\end{verbatim} trailing text`,
	}
	diags := checkBlockOnNewline("t.tex", lines)
	if len(diags) != 0 {
		// `\end{verbatim}` itself is at column 0, so allowed; nothing else
		// after the closer is a block token.
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
