package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MarkAureli/easy-latex/internal/bib"
)

// chdir changes the process working directory to dir and restores it after the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

func writeConfig(t *testing.T, dir, main string, bibFiles ...string) {
	t.Helper()
	elDir := filepath.Join(dir, ".el")
	if err := os.MkdirAll(elDir, 0755); err != nil {
		t.Fatalf("writeConfig mkdir: %v", err)
	}
	cfg := Config{Main: main, BibFiles: bibFiles}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(elDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
}

func skipIfToolMissing(t *testing.T, name string) {
	t.Helper()
	if _, err := findTool(name); err != nil {
		t.Skipf("%s not available", name)
	}
}

// --- Unit tests for detectBibTool ---

func TestDetectBibTool_NoAuxFile(t *testing.T) {
	tool, err := detectBibTool("main", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty", tool)
	}
}

func TestDetectBibTool_NoBib(t *testing.T) {
	auxDir := t.TempDir()
	os.WriteFile(filepath.Join(auxDir, "main.aux"), []byte(`\relax`), 0644)

	tool, err := detectBibTool("main", auxDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty", tool)
	}
}

func TestDetectBibTool_Bibtex(t *testing.T) {
	auxDir := t.TempDir()
	os.WriteFile(filepath.Join(auxDir, "main.aux"), []byte(`\bibdata{refs}`), 0644)

	tool, err := detectBibTool("main", auxDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool != "bibtex" {
		t.Errorf("tool = %q, want %q", tool, "bibtex")
	}
}

func TestDetectBibTool_Biber(t *testing.T) {
	auxDir := t.TempDir()
	os.WriteFile(filepath.Join(auxDir, "main.bcf"), []byte(`<?xml?>`), 0644)

	tool, err := detectBibTool("main", auxDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool != "biber" {
		t.Errorf("tool = %q, want %q", tool, "biber")
	}
}

func TestDetectBibTool_BcfTakesPrecedenceOverAux(t *testing.T) {
	auxDir := t.TempDir()
	// Both present: .bcf should win
	os.WriteFile(filepath.Join(auxDir, "main.bcf"), []byte(`<?xml?>`), 0644)
	os.WriteFile(filepath.Join(auxDir, "main.aux"), []byte(`\bibdata{refs}`), 0644)

	tool, err := detectBibTool("main", auxDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool != "biber" {
		t.Errorf("tool = %q, want %q", tool, "biber")
	}
}

// --- Error case tests (no pdflatex needed) ---

func TestRunCompile_NotInitialized(t *testing.T) {
	chdir(t, t.TempDir())
	if err := runCompile(nil, nil); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunCompile_MissingMainFile(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "missing.tex")
	chdir(t, dir)

	if err := runCompile(nil, nil); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Integration tests ---

const texErrorWithCite = `\documentclass{article}
\begin{document}
A citation~\cite{knuth1984}.
\badcommand
\end{document}
`

const texNoBib = `\documentclass{article}
\begin{document}
Hello world.
\end{document}
`

const texBibtex = `\documentclass{article}
\begin{document}
A citation~\cite{knuth1984}.
\bibliographystyle{plain}
\bibliography{bibliography}
\end{document}
`

const texBiber = `\documentclass{article}
\usepackage[backend=biber]{biblatex}
\addbibresource{bibliography.bib}
\begin{document}
A citation~\cite{knuth1984}.
\printbibliography
\end{document}
`

const bibKnuth = `@book{knuth1984,
  author    = {Donald E. Knuth},
  title     = {The TeXbook},
  year      = {1984},
  publisher = {Addison-Wesley}
}
`

func setupCompileDir(t *testing.T, texContent, bibContent string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.tex"), []byte(texContent), 0644); err != nil {
		t.Fatal(err)
	}
	elDir := filepath.Join(dir, ".el")
	if bibContent != "" {
		bibPath := filepath.Join(dir, "bibliography.bib")
		if err := os.WriteFile(bibPath, []byte(bibContent), 0644); err != nil {
			t.Fatal(err)
		}
		writeConfig(t, dir, "main.tex", "bibliography.bib")
		// Simulate 'el parsebib': seed bib.json, write renames.json, record hash.
		_, renames, err := bib.AllocateCacheEntries([]string{bibPath}, elDir)
		if err != nil {
			t.Fatalf("AllocateCacheEntries: %v", err)
		}
		bib.SaveRenames(elDir, renames)
		bib.UpdateBibHash(bibPath, elDir)
	} else {
		writeConfig(t, dir, "main.tex")
	}
	return dir
}

func assertPDFSymlink(t *testing.T) {
	t.Helper()
	info, err := os.Lstat("main.pdf")
	if err != nil {
		t.Fatalf("main.pdf not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("main.pdf is not a symlink")
	}
	target, _ := os.Readlink("main.pdf")
	if !strings.Contains(target, ".el") {
		t.Errorf("symlink target %q does not point into .el", target)
	}
}

func assertBBLContains(t *testing.T, entry string) {
	t.Helper()
	bbl, err := os.ReadFile(filepath.Join(".el", "main.bbl"))
	if err != nil {
		t.Fatalf("main.bbl not found: %v", err)
	}
	if !strings.Contains(string(bbl), entry) {
		t.Errorf("main.bbl does not contain %q", entry)
	}
}

func TestRunCompile_NoBib(t *testing.T) {
	skipIfToolMissing(t, "pdflatex")
	chdir(t, setupCompileDir(t, texNoBib, ""))

	if err := runCompile(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPDFSymlink(t)
}

func TestRunCompile_Bibtex(t *testing.T) {
	skipIfToolMissing(t, "pdflatex")
	skipIfToolMissing(t, "bibtex")
	chdir(t, setupCompileDir(t, texBibtex, bibKnuth))

	if err := runCompile(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPDFSymlink(t)
	assertBBLContains(t, "Knuth1984TheTexbook")
}

func TestRunCompile_Biber(t *testing.T) {
	skipIfToolMissing(t, "pdflatex")
	skipIfToolMissing(t, "biber")
	chdir(t, setupCompileDir(t, texBiber, bibKnuth))

	if err := runCompile(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPDFSymlink(t)
	assertBBLContains(t, "Knuth1984TheTexbook")
}

func TestRunPdflatex_ErrorSuppressesWarnings(t *testing.T) {
	skipIfToolMissing(t, "pdflatex")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.tex"), []byte(texErrorWithCite), 0644)
	chdir(t, dir)
	os.MkdirAll(auxDir, 0755)

	pdflatex, err := findPdflatex()
	if err != nil {
		t.Fatal(err)
	}

	lines, err := runPdflatex(pdflatex, &Config{Main: "main.tex"})
	if err == nil {
		t.Fatal("expected compilation error")
	}

	if len(lines) == 0 {
		t.Fatal("expected error lines in output, got none")
	}

	for _, line := range lines {
		isError := false
		for _, pat := range errorPatterns {
			if pat.MatchString(line) {
				isError = true
				break
			}
		}
		if !isError {
			t.Errorf("non-error line in output when errors present: %q", line)
		}
	}
}
