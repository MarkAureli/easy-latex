package texscan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanCmds(t *testing.T) {
	src := `\newcommand*{\R}{\mathbb R}
\newcommand\abs[1]{\lvert #1\rvert}
\newcommand{\opt}[2][x]{#1#2}
\def\pair#1#2{(#1,#2)}
\NewDocumentCommand{\xp}{O{a} m +m}{}
\DeclareMathOperator*{\argmax}{arg\,max}
\DeclarePairedDelimiter\norm{\lVert}{\rVert}
\frac{a}{b} and \frac{c}{d}
\sqrt[3]{x} \sqrt{y}
\mathcal{A} \mathcal{B} \mathcal H $\R$ \centering
{\small x}
\hspace*{1cm}
\section{Intro}
% \commented{x}
\begin{verbatim}
\ghost{x}
\end{verbatim}
`
	path := filepath.Join(t.TempDir(), "a.tex")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	got := ScanCmds([]string{path})
	want := map[string]int{
		"R": 0, "abs": 1, "opt": 1, "pair": 2, "xp": 2, "argmax": 0, "norm": 1,
		"frac": 2, "sqrt": 1, "mathcal": 1, "hspace*": 1, "section": 1,
	}
	for k, v := range want {
		if n, ok := got[k]; !ok || n != v {
			t.Errorf("%s: got %d (present %v), want %d", k, n, ok, v)
		}
	}
	for _, k := range []string{"commented", "ghost"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s should be ignored", k)
		}
	}
	decls := ScanCmdDecls([]string{path})
	if _, ok := decls["frac"]; ok || decls["pair"] != 2 {
		t.Errorf("decls: %v", decls)
	}
}
