package pedantic

import "testing"

func TestSentenceTerminator_Basic(t *testing.T) {
	lines := []string{
		`This sentence ends properly.`,         // ok
		`This one is missing punctuation`,       // flag
		`Is this a question`,                    // flag
		`Great, it works!`,                      // ok
	}
	diags := checkSentenceTerminator("t.tex", lines)
	wantLines := []int{2, 3}
	if len(diags) != len(wantLines) {
		t.Fatalf("got %d diags, want %d: %+v", len(diags), len(wantLines), diags)
	}
	for i, want := range wantLines {
		if diags[i].Line != want {
			t.Errorf("diag[%d].Line = %d, want %d (msg=%q)", i, diags[i].Line, want, diags[i].Message)
		}
	}
}

func TestSentenceTerminator_ExemptLeadingTokens(t *testing.T) {
	lines := []string{
		`\section{A Title Without Period}`,
		`\subsection{Another Title}`,
		`\item A fragment without period`,
		`\begin{itemize}`,
		`\end{itemize}`,
		`\[`,
		`\]`,
		`\label{sec:foo}`,
		`\hline`,
		`\title{My Document}`,
	}
	diags := checkSentenceTerminator("t.tex", lines)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags, got %d: %+v", len(diags), diags)
	}
}

func TestSentenceTerminator_CaptionChecked(t *testing.T) {
	lines := []string{
		`\caption{Results for various settings without a period}`,
		`\caption{Results for various settings with a period.}`,
	}
	diags := checkSentenceTerminator("t.tex", lines)
	if len(diags) != 1 || diags[0].Line != 1 {
		t.Fatalf("got %+v, want single diag on line 1", diags)
	}
}

func TestSentenceTerminator_CaptionWithTrailingLabel(t *testing.T) {
	lines := []string{
		`\caption{Results for various settings.}\label{fig:results}`,
		`\caption{Results for various settings}\label{fig:results2}`,
	}
	diags := checkSentenceTerminator("t.tex", lines)
	if len(diags) != 1 || diags[0].Line != 2 {
		t.Fatalf("got %+v, want single diag on line 2", diags)
	}
}

func TestSentenceTerminator_TrailingFootnoteAndCite(t *testing.T) {
	lines := []string{
		`This claim needs support\footnote{See ref.}`,           // flag: outer sentence unterminated
		`This claim is supported.\footnote{See ref.}`,           // ok
		`As shown in prior work\cite{smith2020}`,                // flag
		`As shown in prior work\cite{smith2020}.`,                // ok
		`\footnote{This standalone footnote lacks a period}`,     // flag: checks inside
		`\footnote{This standalone footnote has a period.}`,      // ok
	}
	diags := checkSentenceTerminator("t.tex", lines)
	wantLines := []int{1, 3, 5}
	if len(diags) != len(wantLines) {
		t.Fatalf("got %d diags, want %d: %+v", len(diags), len(wantLines), diags)
	}
	for i, want := range wantLines {
		if diags[i].Line != want {
			t.Errorf("diag[%d].Line = %d, want %d (msg=%q)", i, diags[i].Line, want, diags[i].Message)
		}
	}
}

func TestSentenceTerminator_ColonBeforeListOrMath(t *testing.T) {
	lines := []string{
		`We conclude the following:`,
		`\begin{itemize}`,
		`\item Point one`,
		`\end{itemize}`,
		``,
		`The result is given by:`,
		`\[`,
		`x = y`,
		`\]`,
		``,
		`This colon is not allowed:`,
		`This is just regular text after it.`,
	}
	diags := checkSentenceTerminator("t.tex", lines)
	wantLines := []int{11}
	if len(diags) != len(wantLines) {
		t.Fatalf("got %d diags, want %d: %+v", len(diags), len(wantLines), diags)
	}
	for i, want := range wantLines {
		if diags[i].Line != want {
			t.Errorf("diag[%d].Line = %d, want %d (msg=%q)", i, diags[i].Line, want, diags[i].Message)
		}
	}
}

func TestSentenceTerminator_TrailingLineBreakExempt(t *testing.T) {
	lines := []string{
		`Some address line \\`,
		`continues here without a break.`,
		`Some forced break \newline`,
		`continues here too.`,
	}
	diags := checkSentenceTerminator("t.tex", lines)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags, got %d: %+v", len(diags), diags)
	}
}

func TestSentenceTerminator_MathAndVerbatimSkipped(t *testing.T) {
	lines := []string{
		`\begin{equation}`,
		`x = y + z`,
		`\end{equation}`,
		`\begin{verbatim}`,
		`no period needed here`,
		`\end{verbatim}`,
		`Inline math ends the line $a = b$`, // flag: real text after math needs punctuation
	}
	diags := checkSentenceTerminator("t.tex", lines)
	if len(diags) != 1 || diags[0].Line != 7 {
		t.Fatalf("got %+v, want single diag on line 7", diags)
	}
}

func TestSentenceTerminator_TabularExempt(t *testing.T) {
	lines := []string{
		`\begin{tabular}{cc}`,
		`Name & Value \\`,
		`Total & 42`,
		`\end{tabular}`,
	}
	diags := checkSentenceTerminator("t.tex", lines)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags, got %d: %+v", len(diags), diags)
	}
}

func TestSentenceTerminator_EllipsisAllowed(t *testing.T) {
	lines := []string{
		`And so on\ldots`,
		`And so on…`,
	}
	diags := checkSentenceTerminator("t.tex", lines)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags, got %d: %+v", len(diags), diags)
	}
}
