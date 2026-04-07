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
	data, err := os.ReadFile(filepath.Join(dir, ".el.json"))
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
	if cfg.AuxDir != ".aux_dir" {
		t.Errorf("AuxDir = %q, want %q", cfg.AuxDir, ".aux_dir")
	}
}

func TestDoInit_CreatesAuxDir(t *testing.T) {
	dir := t.TempDir()
	writeTeX(t, dir, "main.tex", `\begin{document}`)

	if err := doInit(dir, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, ".aux_dir"))
	if err != nil {
		t.Fatalf(".aux_dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error(".aux_dir is not a directory")
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
