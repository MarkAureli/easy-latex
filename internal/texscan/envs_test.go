package texscan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanEnvs(t *testing.T) {
	src := `\newenvironment{foo}[2]{a}{b}
\newenvironment{opt}[2][x]{a}{b}
\NewDocumentEnvironment{bar}{o m +m D<>{q} v}{}{}
% \begin{commented}{x}
\begin{tabular}{cc}
a
\end{tabular}
\begin{tabular}{c}\end{tabular}
\begin{tabularx}{\linewidth}{lX}\end{tabularx}
\begin{center}{\large x}\end{center}
\begin{center}
{\large y}\end{center}
\begin{center}
plain
\end{center}
\begin{proof}[Name]
text
\end{proof}
\begin{minipage}[t]{.5\linewidth}\end{minipage}
\begin{verbatim}
\begin{ghost}{x}
\end{verbatim}
\begin{foo}{a}{b}\end{foo}
\newtheorem{lemma}{Lemma}
\newtheorem*{rmk}[lemma]{Remark}
\declaretheorem[style=x]{thm, fact}
`
	path := filepath.Join(t.TempDir(), "a.tex")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	got := ScanEnvs([]string{path})
	want := map[string]int{
		"foo": 2, "opt": 1, "bar": 3, "tabular": 1, "tabularx": 2,
		"center": 0, "proof": 0, "minipage": 1,
	}
	for k, v := range want {
		if got[k].Args != v {
			t.Errorf("%s: got %d, want %d", k, got[k].Args, v)
		}
	}
	for _, k := range []string{"commented", "ghost"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s should be ignored", k)
		}
	}
	for _, k := range []string{"lemma", "rmk", "thm", "fact"} {
		if !got[k].Theorem {
			t.Errorf("%s should be a theorem", k)
		}
	}
	if got["tabular"].Theorem {
		t.Error("tabular is not a theorem")
	}
}
