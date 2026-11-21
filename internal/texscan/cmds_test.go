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
	got := ScanCmds([]string{path}, nil)
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
	decls, _ := ScanCmdDecls([]string{path})
	if _, ok := decls["frac"]; ok || decls["pair"] != 2 {
		t.Errorf("decls: %v", decls)
	}
}

func TestResolveCmd(t *testing.T) {
	declared := map[string]int{"foo": 2, "pfoo": 1, "pbar": 1}
	provided := map[string]bool{"pfoo": true, "pbar": true}
	probed := map[string]int{"foo": 3, "pfoo": 2, "frac": 3, "ref": 1}
	usage := map[string]int{"frac": 0, "sqrt": 0, "mine": 2, "ref": 1}
	for name, want := range map[string]int{
		"foo":   2, // declaration beats probe
		"pfoo":  2, // \providecommand yields to the probe
		"pbar":  1, // ...but stands when the probe did not resolve
		"frac":  3, // probe beats built-in
		"sqrt":  1, // built-in beats usage
		"mine":  2, // usage as last resort
		"other": 0,
	} {
		if got := ResolveCmd(name, declared, provided, probed, usage); got != want {
			t.Errorf("%s: got %d, want %d", name, got, want)
		}
	}
}

func TestScanCmdsProbedAndProvided(t *testing.T) {
	src := "\\providecommand\\pf[1]{}\n\\newcommand\\nf[1]{}\\providecommand\\nf[2]{}\n\\frac12 \\frac34 \\pf{x} \\nf{x}\n"
	path := filepath.Join(t.TempDir(), "a.tex")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	got := ScanCmds([]string{path}, map[string]int{"pf": 3, "nf": 3})
	if got["frac"] != 2 || got["pf"] != 3 || got["nf"] != 2 {
		t.Errorf("got %v", got)
	}
	_, provided := ScanCmdDecls([]string{path})
	if !provided["pf"] || provided["nf"] {
		t.Errorf("provided: %v", provided)
	}
}
