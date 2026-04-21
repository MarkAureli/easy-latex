package texscan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTex writes a .tex file into dir.
func writeTex(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeTex: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile %s: %v", path, err)
	}
	return string(data)
}

// --- ResolveFileContents ---

func TestResolveFileContents_NoBlock(t *testing.T) {
	dir := t.TempDir()
	original := "\\begin{document}\nHello\n\\end{document}\n"
	writeTex(t, dir, "main.tex", original)

	if err := ResolveFileContents("main.tex", dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// tex file unchanged
	got := readFile(t, filepath.Join(dir, "main.tex"))
	if got != original {
		t.Errorf("tex file modified unexpectedly:\n%s", got)
	}
	// no .bib written
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".bib") {
			t.Errorf("unexpected bib file created: %s", e.Name())
		}
	}
}

func TestResolveFileContents_BasicBlock(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{filecontents}{refs.bib}
@article{key,
  author = {Smith, J.},
}
\end{filecontents}
\begin{document}
\bibliography{refs}
\end{document}
`)

	if err := ResolveFileContents("main.tex", dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// bib file created with correct content
	bib := readFile(t, filepath.Join(dir, "refs.bib"))
	if !strings.Contains(bib, "@article{key") {
		t.Errorf("bib content missing: %s", bib)
	}

	// block removed from tex
	tex := readFile(t, filepath.Join(dir, "main.tex"))
	if strings.Contains(tex, `\begin{filecontents}`) {
		t.Error("filecontents block not removed from tex")
	}
	if !strings.Contains(tex, `\bibliography{refs}`) {
		t.Error("\\bibliography declaration removed unexpectedly")
	}
}

func TestResolveFileContents_AsteriskVariant(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{filecontents*}{refs.bib}
@misc{k, title = {T}}
\end{filecontents*}
\begin{document}\end{document}
`)

	if err := ResolveFileContents("main.tex", dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bib := readFile(t, filepath.Join(dir, "refs.bib"))
	if !strings.Contains(bib, "@misc{k") {
		t.Errorf("bib content missing: %s", bib)
	}
	tex := readFile(t, filepath.Join(dir, "main.tex"))
	if strings.Contains(tex, `\begin{filecontents*}`) {
		t.Error("filecontents* block not removed from tex")
	}
}

func TestResolveFileContents_ForceOption(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{filecontents}[force]{refs.bib}
@book{b, title = {B}}
\end{filecontents}
\begin{document}\end{document}
`)

	if err := ResolveFileContents("main.tex", dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bib := readFile(t, filepath.Join(dir, "refs.bib"))
	if !strings.Contains(bib, "@book{b") {
		t.Errorf("bib content missing: %s", bib)
	}
	tex := readFile(t, filepath.Join(dir, "main.tex"))
	if strings.Contains(tex, `\begin{filecontents}`) {
		t.Error("filecontents block not removed from tex")
	}
}

func TestResolveFileContents_OverwriteOption(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{filecontents}[overwrite]{refs.bib}
@book{b2, title = {B2}}
\end{filecontents}
\begin{document}\end{document}
`)

	if err := ResolveFileContents("main.tex", dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bib := readFile(t, filepath.Join(dir, "refs.bib"))
	if !strings.Contains(bib, "@book{b2") {
		t.Errorf("bib content missing: %s", bib)
	}
}

func TestResolveFileContents_MultipleBlocksSameFile(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{filecontents}{a.bib}
@article{a1, title = {A}}
\end{filecontents}
\begin{filecontents}{b.bib}
@article{b1, title = {B}}
\end{filecontents}
\begin{document}\end{document}
`)

	if err := ResolveFileContents("main.tex", dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(readFile(t, filepath.Join(dir, "a.bib")), "@article{a1") {
		t.Error("a.bib content wrong")
	}
	if !strings.Contains(readFile(t, filepath.Join(dir, "b.bib")), "@article{b1") {
		t.Error("b.bib content wrong")
	}
	tex := readFile(t, filepath.Join(dir, "main.tex"))
	if strings.Contains(tex, `\begin{filecontents}`) {
		t.Error("filecontents blocks not fully removed")
	}
}

func TestResolveFileContents_BlockInIncludedFile(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\input{sub}
\begin{document}\end{document}
`)
	writeTex(t, dir, "sub.tex", `\begin{filecontents}{refs.bib}
@article{x, title = {X}}
\end{filecontents}
`)

	if err := ResolveFileContents("main.tex", dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bib := readFile(t, filepath.Join(dir, "refs.bib"))
	if !strings.Contains(bib, "@article{x") {
		t.Errorf("bib content missing: %s", bib)
	}
	sub := readFile(t, filepath.Join(dir, "sub.tex"))
	if strings.Contains(sub, `\begin{filecontents}`) {
		t.Error("filecontents block not removed from included file")
	}
}

func TestResolveFileContents_FindBibFilesFindsResolvedBib(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{filecontents}{refs.bib}
@article{a, title = {A}}
\end{filecontents}
\begin{document}
\bibliography{refs}
\end{document}
`)

	if err := ResolveFileContents("main.tex", dir); err != nil {
		t.Fatalf("ResolveFileContents: %v", err)
	}

	bibs := FindBibFiles("main.tex", dir)
	if len(bibs) != 1 || bibs[0] != "refs.bib" {
		t.Errorf("FindBibFiles = %v, want [refs.bib]", bibs)
	}
}

// --- RewriteBibReferences ---

func TestRewriteBibReferences_Bibliography(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\documentclass{article}
\begin{document}
\bibliography{refs}
\end{document}
`)

	if err := RewriteBibReferences("main.tex", dir, []string{"bibliography.bib"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, filepath.Join(dir, "main.tex"))
	if strings.Contains(got, `\bibliography{refs}`) {
		t.Error(`\bibliography{refs} not replaced`)
	}
	if !strings.Contains(got, `\bibliography{bibliography}`) {
		t.Errorf("\\bibliography{bibliography} not found in tex:\n%s", got)
	}
}

func TestRewriteBibReferences_AddBibResource(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\usepackage[backend=biber]{biblatex}
\addbibresource{refs.bib}
\begin{document}
\end{document}
`)

	if err := RewriteBibReferences("main.tex", dir, []string{"bibliography.bib"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, filepath.Join(dir, "main.tex"))
	if strings.Contains(got, `\addbibresource{refs.bib}`) {
		t.Error(`\addbibresource{refs.bib} not replaced`)
	}
	if !strings.Contains(got, `\addbibresource{bibliography.bib}`) {
		t.Errorf("\\addbibresource{bibliography.bib} not found in tex:\n%s", got)
	}
}

func TestRewriteBibReferences_TwoBibFiles(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{document}
\bibliography{a,b}
\end{document}
`)

	if err := RewriteBibReferences("main.tex", dir, []string{"preamble.bib", "bibliography.bib"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, filepath.Join(dir, "main.tex"))
	if !strings.Contains(got, `\bibliography{preamble,bibliography}`) {
		t.Errorf("expected \\bibliography{preamble,bibliography}, got:\n%s", got)
	}
}

func TestRewriteBibReferences_TwoAddBibResources(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\addbibresource{refs.bib}
\begin{document}
\end{document}
`)

	if err := RewriteBibReferences("main.tex", dir, []string{"preamble.bib", "bibliography.bib"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, filepath.Join(dir, "main.tex"))
	if strings.Count(got, `\addbibresource{`) != 2 {
		t.Errorf("expected 2 \\addbibresource lines, got:\n%s", got)
	}
	if !strings.Contains(got, `\addbibresource{preamble.bib}`) {
		t.Errorf("preamble.bib addbibresource missing:\n%s", got)
	}
	if !strings.Contains(got, `\addbibresource{bibliography.bib}`) {
		t.Errorf("bibliography.bib addbibresource missing:\n%s", got)
	}
}

func TestRewriteBibReferences_NoOp(t *testing.T) {
	dir := t.TempDir()
	original := `\begin{document}
Hello world.
\end{document}
`
	writeTex(t, dir, "main.tex", original)

	if err := RewriteBibReferences("main.tex", dir, []string{"bibliography.bib"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, filepath.Join(dir, "main.tex"))
	if got != original {
		t.Errorf("file modified unexpectedly:\n%s", got)
	}
}

func TestRewriteBibReferences_DuplicateBibliographyDropped(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\begin{document}
\bibliography{refs}
\bibliography{refs}
\end{document}
`)

	if err := RewriteBibReferences("main.tex", dir, []string{"bibliography.bib"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, filepath.Join(dir, "main.tex"))
	if strings.Count(got, `\bibliography{`) != 1 {
		t.Errorf("expected exactly one \\bibliography, got:\n%s", got)
	}
}

func TestRewriteBibReferences_InIncludedFile(t *testing.T) {
	dir := t.TempDir()
	writeTex(t, dir, "main.tex", `\input{sub}
\begin{document}\end{document}
`)
	writeTex(t, dir, "sub.tex", `\bibliography{refs}
`)

	if err := RewriteBibReferences("main.tex", dir, []string{"bibliography.bib"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, filepath.Join(dir, "sub.tex"))
	if !strings.Contains(got, `\bibliography{bibliography}`) {
		t.Errorf("expected \\bibliography{bibliography} in sub.tex, got:\n%s", got)
	}
}

func TestRewriteBibReferences_CommentedBibliographyIgnored(t *testing.T) {
	dir := t.TempDir()
	original := `\begin{document}
% \bibliography{refs}
\end{document}
`
	writeTex(t, dir, "main.tex", original)

	if err := RewriteBibReferences("main.tex", dir, []string{"bibliography.bib"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readFile(t, filepath.Join(dir, "main.tex"))
	if got != original {
		t.Errorf("commented \\bibliography should not be rewritten, got:\n%s", got)
	}
}

func TestResolveFileContents_CommentedOutBlock(t *testing.T) {
	dir := t.TempDir()
	original := `% \begin{filecontents}{refs.bib}
% @article{a, title = {A}}
% \end{filecontents}
\begin{document}\end{document}
`
	writeTex(t, dir, "main.tex", original)

	if err := ResolveFileContents("main.tex", dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// commented block must NOT trigger bib creation or tex modification
	got := readFile(t, filepath.Join(dir, "main.tex"))
	if got != original {
		t.Error("tex file modified for commented-out block")
	}
	if _, err := os.Stat(filepath.Join(dir, "refs.bib")); err == nil {
		t.Error("refs.bib created for commented-out block")
	}
}

// --- StripComment ---

func TestStripComment_EscapedPercent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Cost is 50\\% of total",
			expected: "Cost is 50\\% of total",
		},
		{
			input:    "Price: \\$100\\% markup",
			expected: "Price: \\$100\\% markup",
		},
		{
			input:    "Discount \\% % this is a comment",
			expected: "Discount \\% ",
		},
		{
			input:    "No percent sign here",
			expected: "No percent sign here",
		},
		{
			input:    "Regular comment % this is stripped",
			expected: "Regular comment ",
		},
		{
			input:    "Multiple \\% signs \\% preserved",
			expected: "Multiple \\% signs \\% preserved",
		},
	}

	for _, tc := range tests {
		got := StripComment(tc.input)
		if got != tc.expected {
			t.Errorf("StripComment(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
