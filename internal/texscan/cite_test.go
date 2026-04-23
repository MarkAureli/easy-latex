package texscan

import (
	"testing"
)

func TestFindCiteKeys_BasicCite(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{document}
Some text~\cite{Smith2024}.
\end{document}
`)

	got := FindCiteKeys("main.tex", dir)
	if len(got) != 1 || got[0] != "Smith2024" {
		t.Errorf("FindCiteKeys = %v, want [Smith2024]", got)
	}
}

func TestFindCiteKeys_MultiplKeysInOneCite(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{document}
\cite{Alpha2024, Beta2023,Gamma2022}
\end{document}
`)

	got := FindCiteKeys("main.tex", dir)
	want := []string{"Alpha2024", "Beta2023", "Gamma2022"}
	if len(got) != len(want) {
		t.Fatalf("FindCiteKeys = %v, want %v", got, want)
	}
	for i, k := range want {
		if got[i] != k {
			t.Errorf("got[%d] = %q, want %q", i, got[i], k)
		}
	}
}

func TestFindCiteKeys_Variants(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{document}
\citep{A}
\citet{B}
\citeauthor{C}
\parencite{D}
\textcite{E}
\autocite{F}
\fullcite{G}
\cite*{H}
\citep*{I}
\Cite{J}
\Citep{K}
\citealt{L}
\citealp{M}
\Citet{N}
\end{document}
`)

	got := FindCiteKeys("main.tex", dir)
	want := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}
	if len(got) != len(want) {
		t.Fatalf("FindCiteKeys = %v, want %v", got, want)
	}
	for i, k := range want {
		if got[i] != k {
			t.Errorf("got[%d] = %q, want %q", i, got[i], k)
		}
	}
}

func TestFindCiteKeys_OptionalArgs(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{document}
\cite[p.~42]{Smith2024}
\citep[see][chap.~2]{Jones2023}
\end{document}
`)

	got := FindCiteKeys("main.tex", dir)
	want := []string{"Jones2023", "Smith2024"}
	if len(got) != len(want) {
		t.Fatalf("FindCiteKeys = %v, want %v", got, want)
	}
	for i, k := range want {
		if got[i] != k {
			t.Errorf("got[%d] = %q, want %q", i, got[i], k)
		}
	}
}

func TestFindCiteKeys_Dedup(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{document}
\cite{Smith2024}
\cite{Smith2024}
\citep{Smith2024}
\end{document}
`)

	got := FindCiteKeys("main.tex", dir)
	if len(got) != 1 || got[0] != "Smith2024" {
		t.Errorf("FindCiteKeys = %v, want [Smith2024]", got)
	}
}

func TestFindCiteKeys_Sorted(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{document}
\cite{Zebra2024}
\cite{Alpha2020}
\cite{Middle2022}
\end{document}
`)

	got := FindCiteKeys("main.tex", dir)
	want := []string{"Alpha2020", "Middle2022", "Zebra2024"}
	if len(got) != len(want) {
		t.Fatalf("FindCiteKeys = %v, want %v", got, want)
	}
	for i, k := range want {
		if got[i] != k {
			t.Errorf("got[%d] = %q, want %q", i, got[i], k)
		}
	}
}

func TestFindCiteKeys_CommentedOut(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{document}
\cite{Real2024}
% \cite{Commented2024}
\end{document}
`)

	got := FindCiteKeys("main.tex", dir)
	if len(got) != 1 || got[0] != "Real2024" {
		t.Errorf("FindCiteKeys = %v, want [Real2024]", got)
	}
}

func TestFindCiteKeys_IncludedFile(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\input{chapter1}
\begin{document}\end{document}
`)
	writeTex(t, dir, "chapter1.tex", `\cite{FromIncluded2024}
`)

	got := FindCiteKeys("main.tex", dir)
	if len(got) != 1 || got[0] != "FromIncluded2024" {
		t.Errorf("FindCiteKeys = %v, want [FromIncluded2024]", got)
	}
}

func TestFindCiteKeys_Empty(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{document}
No citations here.
\end{document}
`)

	got := FindCiteKeys("main.tex", dir)
	if len(got) != 0 {
		t.Errorf("FindCiteKeys = %v, want empty", got)
	}
}

func TestFindCiteKeys_NoTexFiles(t *testing.T) {
	dir := t.TempDir()

	got := FindCiteKeys("nonexistent.tex", dir)
	if len(got) != 0 {
		t.Errorf("FindCiteKeys = %v, want empty", got)
	}
}

func TestFindCiteKeys_EscapedPercent(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{document}
Cost is 50\% of \cite{Smith2024}.
\end{document}
`)

	got := FindCiteKeys("main.tex", dir)
	if len(got) != 1 || got[0] != "Smith2024" {
		t.Errorf("FindCiteKeys = %v, want [Smith2024]", got)
	}
}
