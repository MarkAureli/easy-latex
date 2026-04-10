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
	err := doInit(dir, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDoInit_TexFileWithoutBeginDocument(t *testing.T) {
	dir := t.TempDir()
	writeTeX(t, dir, "fragment.tex", `\section{Intro}`)
	err := doInit(dir, nil)
	if err == nil {
		t.Fatal("expected error for tex file without \\begin{document}, got nil")
	}
}

func TestDoInit_OneMainFile(t *testing.T) {
	dir := t.TempDir()
	writeTeX(t, dir, "main.tex", `\documentclass{article}`+"\n"+`\begin{document}`+"\n"+`Hello`+"\n"+`\end{document}`)

	if err := doInit(dir, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := readConfig(t, dir)
	if cfg.Main != "main.tex" {
		t.Errorf("Main = %q, want %q", cfg.Main, "main.tex")
	}
	if cfg.AuxDir != ".el" {
		t.Errorf("AuxDir = %q, want %q", cfg.AuxDir, ".el")
	}
}

func TestDoInit_CreatesElDir(t *testing.T) {
	dir := t.TempDir()
	writeTeX(t, dir, "main.tex", `\begin{document}`)

	if err := doInit(dir, nil); err != nil {
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

	err := doInit(dir, nil)
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
	if err := doInit(dir, stdin); err != nil {
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
	if err := doInit(dir, stdin); err != nil {
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

	if err := doInit(dir, nil); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if err := doInit(dir, nil); err != nil {
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

func TestDoInit_UpdatesGitExclude(t *testing.T) {
	dir := t.TempDir()
	gitDir := makeGitRepo(t, dir)
	writeTeX(t, dir, "main.tex", `\begin{document}`)

	if err := doInit(dir, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := readExclude(t, gitDir)
	if !strings.Contains(content, ".el") {
		t.Errorf("exclude missing %q after init", ".el")
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
