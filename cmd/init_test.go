package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTeX writes a .tex file into dir with the given content.
func writeTeX(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeTeX: %v", err)
	}
}

func readConfig(t *testing.T, dir string) Config {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".el", "config.json"))
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("readConfig unmarshal: %v", err)
	}
	return cfg
}

func TestDoInit_NoTexFiles(t *testing.T) {
	dir := t.TempDir()
	err := doInit(dir, nil, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDoInit_TexFileWithoutBeginDocument(t *testing.T) {
	dir := t.TempDir()
	writeTeX(t, dir, "fragment.tex", `\section{Intro}`)
	err := doInit(dir, nil, false)
	if err == nil {
		t.Fatal("expected error for tex file without \\begin{document}, got nil")
	}
}

func TestDoInit_OneMainFile(t *testing.T) {
	dir := t.TempDir()
	writeTeX(t, dir, "main.tex", `\documentclass{article}`+"\n"+`\begin{document}`+"\n"+`Hello`+"\n"+`\end{document}`)

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := readConfig(t, dir)
	if cfg.Main != "main.tex" {
		t.Errorf("Main = %q, want %q", cfg.Main, "main.tex")
	}

}

func TestDoInit_CreatesElDir(t *testing.T) {
	dir := t.TempDir()
	writeTeX(t, dir, "main.tex", `\begin{document}`)

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, ".el"))
	if err != nil {
		t.Fatalf(".el not created: %v", err)
	}
	if !info.IsDir() {
		t.Error(".el is not a directory")
	}
}

func TestDoInit_SubdirTexIgnored(t *testing.T) {
	dir := t.TempDir()
	// tex file in a subdirectory — should not be detected
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeTeX(t, sub, "nested.tex", `\begin{document}`)

	err := doInit(dir, nil, false)
	if err == nil {
		t.Fatal("expected error when only subdir contains tex file, got nil")
	}
}

func TestDoInit_MultipleMainFiles_PicksCorrect(t *testing.T) {
	dir := t.TempDir()
	writeTeX(t, dir, "aaa.tex", `\begin{document}`)
	writeTeX(t, dir, "bbb.tex", `\begin{document}`)

	// Simulate user entering "2" to pick bbb.tex
	stdin := strings.NewReader("2\n")
	if err := doInit(dir, stdin, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := readConfig(t, dir)
	if cfg.Main != "bbb.tex" {
		t.Errorf("Main = %q, want %q", cfg.Main, "bbb.tex")
	}
}

func TestDoInit_MultipleMainFiles_InvalidThenValid(t *testing.T) {
	dir := t.TempDir()
	writeTeX(t, dir, "aaa.tex", `\begin{document}`)
	writeTeX(t, dir, "bbb.tex", `\begin{document}`)

	// First input is invalid, second is valid
	stdin := strings.NewReader("99\n1\n")
	if err := doInit(dir, stdin, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := readConfig(t, dir)
	if cfg.Main != "aaa.tex" {
		t.Errorf("Main = %q, want %q", cfg.Main, "aaa.tex")
	}
}

func TestDoInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	writeTeX(t, dir, "main.tex", `\begin{document}`)

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("second init: %v", err)
	}

	cfg := readConfig(t, dir)
	if cfg.Main != "main.tex" {
		t.Errorf("Main = %q, want %q", cfg.Main, "main.tex")
	}
}

// --- Tests for updateGitExclude ---

func makeGitRepo(t *testing.T, dir string) string {
	t.Helper()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "info"), 0755); err != nil {
		t.Fatalf("makeGitRepo: %v", err)
	}
	return gitDir
}

func readExclude(t *testing.T, gitDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	if err != nil {
		t.Fatalf("readExclude: %v", err)
	}
	return string(data)
}

func TestUpdateGitExclude_NoGitDir(t *testing.T) {
	// Should succeed silently when there is no .git directory
	if err := updateGitExclude(t.TempDir()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateGitExclude_AddsEntries(t *testing.T) {
	dir := t.TempDir()
	gitDir := makeGitRepo(t, dir)

	if err := updateGitExclude(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readExclude(t, gitDir)
	if !strings.Contains(content, ".el") {
		t.Errorf("exclude missing %q", ".el")
	}
}

func TestUpdateGitExclude_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	gitDir := makeGitRepo(t, dir)

	// Pre-populate exclude with the entry already present
	os.WriteFile(filepath.Join(gitDir, "info", "exclude"), []byte(".el\n"), 0644)

	if err := updateGitExclude(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readExclude(t, gitDir)
	if strings.Count(content, ".el") != 1 {
		t.Errorf(".el appears more than once in exclude:\n%s", content)
	}
}

func TestUpdateGitExclude_AddsToEmptyFile(t *testing.T) {
	dir := t.TempDir()
	gitDir := makeGitRepo(t, dir)

	// Pre-populate with an empty file
	os.WriteFile(filepath.Join(gitDir, "info", "exclude"), []byte(""), 0644)

	if err := updateGitExclude(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readExclude(t, gitDir)
	if strings.Count(content, ".el") != 1 {
		t.Errorf(".el should appear exactly once in exclude:\n%s", content)
	}
}

func TestUpdateGitExclude_NoTrailingNewlineHandled(t *testing.T) {
	dir := t.TempDir()
	gitDir := makeGitRepo(t, dir)

	// File exists but has no trailing newline
	os.WriteFile(filepath.Join(gitDir, "info", "exclude"), []byte("# comment"), 0644)

	if err := updateGitExclude(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readExclude(t, gitDir)
	// Entry must appear as a whole line, not concatenated with "# comment"
	for _, line := range strings.Split(content, "\n") {
		if line == "# comment.el" {
			t.Errorf("entry was concatenated with existing content: %q", line)
		}
	}
	if !strings.Contains(content, ".el") {
		t.Errorf("exclude missing .el")
	}
}

func TestUpdateGitExclude_GitInParentDir(t *testing.T) {
	root := t.TempDir()
	gitDir := makeGitRepo(t, root)

	sub := filepath.Join(root, "sub", "proj")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := updateGitExclude(sub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readExclude(t, gitDir)
	want := filepath.Join("sub", "proj", ".el")
	if !strings.Contains(content, want) {
		t.Errorf("exclude missing %q, got:\n%s", want, content)
	}
}

func TestDoInit_UpdatesGitExclude(t *testing.T) {
	dir := t.TempDir()
	gitDir := makeGitRepo(t, dir)
	writeTeX(t, dir, "main.tex", `\begin{document}`)

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readExclude(t, gitDir)
	if !strings.Contains(content, ".el") {
		t.Errorf("exclude missing %q after init", ".el")
	}
}

// readTestFile reads the content of a file for use in assertions.
func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readTestFile %s: %v", path, err)
	}
	return string(data)
}

// bibBook is a simple book entry without DOI/arXiv — avoids network calls in AllocateCacheEntries.
const bibBook = `@book{knuth1984,
  author    = {Knuth, Donald E.},
  year      = {1984},
  title     = {The TeXbook},
  publisher = {Addison-Wesley},
}
`

func TestDoInit_MainBibTransferredBeforeCache(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeTeX(t, dir, "main.tex", "\\documentclass{article}\n\\begin{document}\n\\bibliography{main}\n\\end{document}\n")
	if err := os.WriteFile(filepath.Join(dir, "main.bib"), []byte(bibBook), 0644); err != nil {
		t.Fatal(err)
	}

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// bibliography.bib must exist with the entry
	ref := readTestFile(t, filepath.Join(dir, "bibliography.bib"))
	if !strings.Contains(ref, "@book") {
		t.Errorf("bibliography.bib missing @book entry:\n%s", ref)
	}
	// original main.bib deleted
	if _, err := os.Stat(filepath.Join(dir, "main.bib")); err == nil {
		t.Error("main.bib should be deleted after condensation")
	}
	// config registers bibliography.bib
	cfg := readConfig(t, dir)
	if len(cfg.BibFiles) != 1 || cfg.BibFiles[0] != "bibliography.bib" {
		t.Errorf("BibFiles = %v, want [bibliography.bib]", cfg.BibFiles)
	}
	// bib.json cache seeded — proves content was transferred before AllocateCacheEntries
	if _, err := os.Stat(filepath.Join(dir, ".el", "bib.json")); err != nil {
		t.Errorf("bib.json not created — AllocateCacheEntries may not have run: %v", err)
	}
}

func TestDoInit_BibFilesCondensed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeTeX(t, dir, "main.tex", "\\documentclass{article}\n\\begin{document}\n\\bibliography{refs}\n\\end{document}\n")
	if err := os.WriteFile(filepath.Join(dir, "refs.bib"), []byte(bibBook), 0644); err != nil {
		t.Fatal(err)
	}

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// bibliography.bib created with the entry
	ref := readTestFile(t, filepath.Join(dir, "bibliography.bib"))
	if !strings.Contains(ref, "@book") {
		t.Errorf("bibliography.bib missing @book entry:\n%s", ref)
	}
	// preamble.bib not created (no @string/@preamble)
	if _, err := os.Stat(filepath.Join(dir, "preamble.bib")); err == nil {
		t.Error("preamble.bib should not be created when there is no preamble content")
	}
	// original deleted
	if _, err := os.Stat(filepath.Join(dir, "refs.bib")); err == nil {
		t.Error("original refs.bib should be deleted after condensation")
	}
}

func TestDoInit_BibPreambleSplit(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeTeX(t, dir, "main.tex", "\\documentclass{article}\n\\begin{document}\n\\bibliography{refs}\n\\end{document}\n")
	content := "@string{pub = {Addison-Wesley}}\n\n" + bibBook
	if err := os.WriteFile(filepath.Join(dir, "refs.bib"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pre := readTestFile(t, filepath.Join(dir, "preamble.bib"))
	if !strings.Contains(pre, "@string") {
		t.Errorf("preamble.bib missing @string:\n%s", pre)
	}
	ref := readTestFile(t, filepath.Join(dir, "bibliography.bib"))
	if !strings.Contains(ref, "@book") {
		t.Errorf("bibliography.bib missing @book:\n%s", ref)
	}
	if strings.Contains(ref, "@string") {
		t.Error("bibliography.bib should not contain @string")
	}
	// preamble listed first in config
	cfg := readConfig(t, dir)
	if len(cfg.BibFiles) != 2 || cfg.BibFiles[0] != "preamble.bib" || cfg.BibFiles[1] != "bibliography.bib" {
		t.Errorf("BibFiles = %v, want [preamble.bib bibliography.bib]", cfg.BibFiles)
	}
}

func TestDoInit_BibCommentsDropped(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeTeX(t, dir, "main.tex", "\\begin{document}\n\\bibliography{refs}\n\\end{document}\n")
	content := "% top-level comment\n@string{pub = {Addison-Wesley}}\n" + bibBook
	if err := os.WriteFile(filepath.Join(dir, "refs.bib"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pre := readTestFile(t, filepath.Join(dir, "preamble.bib"))
	if strings.Contains(pre, "% top-level comment") {
		t.Error("preamble.bib should not contain % comment lines")
	}
}

func TestDoInit_BibAtCommentDropped(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeTeX(t, dir, "main.tex", "\\begin{document}\n\\bibliography{refs}\n\\end{document}\n")
	content := "@comment{This should be dropped}\n" + bibBook
	if err := os.WriteFile(filepath.Join(dir, "refs.bib"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ref := readTestFile(t, filepath.Join(dir, "bibliography.bib"))
	if strings.Contains(ref, "@comment") {
		t.Error("bibliography.bib should not contain @comment blocks")
	}
	if _, err := os.Stat(filepath.Join(dir, "preamble.bib")); err == nil {
		t.Error("preamble.bib should not be created when only @comment was present")
	}
}

func TestDoInit_BibliographyRewritten(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeTeX(t, dir, "main.tex", "\\documentclass{article}\n\\begin{document}\n\\bibliography{refs}\n\\end{document}\n")
	if err := os.WriteFile(filepath.Join(dir, "refs.bib"), []byte(bibBook), 0644); err != nil {
		t.Fatal(err)
	}

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tex := readTestFile(t, filepath.Join(dir, "main.tex"))
	if strings.Contains(tex, `\bibliography{refs}`) {
		t.Error(`\bibliography{refs} not rewritten in main.tex`)
	}
	if !strings.Contains(tex, `\bibliography{bibliography}`) {
		t.Errorf("\\bibliography{bibliography} not found in main.tex:\n%s", tex)
	}
}

func TestDoInit_MultipleBibFilesCondensed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeTeX(t, dir, "main.tex", "\\documentclass{article}\n\\begin{document}\n\\bibliography{a,b}\n\\end{document}\n")
	if err := os.WriteFile(filepath.Join(dir, "a.bib"), []byte(bibBook), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.bib"), []byte("@book{extra,\n  author = {A, B},\n  year   = {2000},\n  title  = {Extra},\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ref := readTestFile(t, filepath.Join(dir, "bibliography.bib"))
	if !strings.Contains(ref, "@book") {
		t.Errorf("bibliography.bib missing entries:\n%s", ref)
	}
	cfg := readConfig(t, dir)
	if len(cfg.BibFiles) != 1 || cfg.BibFiles[0] != "bibliography.bib" {
		t.Errorf("BibFiles = %v, want [bibliography.bib]", cfg.BibFiles)
	}
}

func TestDoInit_BibIdempotent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeTeX(t, dir, "main.tex", "\\documentclass{article}\n\\begin{document}\n\\bibliography{refs}\n\\end{document}\n")
	if err := os.WriteFile(filepath.Join(dir, "refs.bib"), []byte(bibBook), 0644); err != nil {
		t.Fatal(err)
	}

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("first init: %v", err)
	}
	ref1 := readTestFile(t, filepath.Join(dir, "bibliography.bib"))

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("second init: %v", err)
	}
	ref2 := readTestFile(t, filepath.Join(dir, "bibliography.bib"))

	if ref1 != ref2 {
		t.Errorf("bibliography.bib changed after second init:\nbefore:\n%s\nafter:\n%s", ref1, ref2)
	}
}

func TestDoInit_IEEEFlag_FileNames(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeTeX(t, dir, "main.tex", "\\documentclass{article}\n\\begin{document}\n\\bibliography{refs}\n\\end{document}\n")
	content := "@string{pub = {Addison-Wesley}}\n\n" + bibBook
	if err := os.WriteFile(filepath.Join(dir, "refs.bib"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := doInit(dir, nil, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// entries → bibliography.bib
	ref := readTestFile(t, filepath.Join(dir, "bibliography.bib"))
	if !strings.Contains(ref, "@book") {
		t.Errorf("bibliography.bib missing @book entry:\n%s", ref)
	}
	// preamble → IEEEabrv.bib
	pre := readTestFile(t, filepath.Join(dir, "IEEEabrv.bib"))
	if !strings.Contains(pre, "@string") {
		t.Errorf("IEEEabrv.bib missing @string:\n%s", pre)
	}
	// preamble.bib must not be created when --ieee is set
	if _, err := os.Stat(filepath.Join(dir, "preamble.bib")); err == nil {
		t.Error("preamble.bib must not be created when --ieee is set")
	}
	// config bib_files order: IEEEabrv.bib first
	cfg := readConfig(t, dir)
	if len(cfg.BibFiles) != 2 || cfg.BibFiles[0] != "IEEEabrv.bib" || cfg.BibFiles[1] != "bibliography.bib" {
		t.Errorf("BibFiles = %v, want [IEEEabrv.bib bibliography.bib]", cfg.BibFiles)
	}
}

func TestDoInit_IEEEFlag_SetsIEEEFormat(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeTeX(t, dir, "main.tex", "\\begin{document}\n\\end{document}\n")

	if err := doInit(dir, nil, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := readConfig(t, dir)
	if cfg.Bib.IEEEFormat == nil || !*cfg.Bib.IEEEFormat {
		t.Error("IEEEFormat should be true in config when --ieee flag is set")
	}
}

func TestDoInit_NoIEEE_IEEEFormatUnset(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeTeX(t, dir, "main.tex", "\\begin{document}\n\\end{document}\n")

	if err := doInit(dir, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := readConfig(t, dir)
	if cfg.Bib.IEEEFormat != nil {
		t.Errorf("IEEEFormat should be nil (unset) when --ieee flag is absent, got %v", *cfg.Bib.IEEEFormat)
	}
}

func TestHasBeginDocument(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		content string
		want    bool
	}{
		{`\begin{document}`, true},
		{`some text` + "\n" + `\begin{document}` + "\n" + `more`, true},
		{`\documentclass{article}`, false},
		{``, false},
	}

	for _, tc := range cases {
		path := filepath.Join(dir, "test.tex")
		if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
			t.Fatal(err)
		}
		got := hasBeginDocument(path)
		if got != tc.want {
			t.Errorf("hasBeginDocument(%q) = %v, want %v", tc.content, got, tc.want)
		}
	}
}
