package pedantic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSectionLinebreakDetect(t *testing.T) {
	dir := t.TempDir()
	stem := "main"
	posPath := filepath.Join(dir, stem+".sectionpos")
	if err := os.WriteFile(filepath.Join(dir, stem+".tex"), []byte("\\section{A}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// id 1: same y → ok
	// id 2: differing y → violation, kind=section, line=42
	// id 3: missing E → ignored
	content := "" +
		"M 1 10 section\n" +
		"S 1 1000000 section\n" +
		"E 1 1000000 section\n" +
		"M 2 42 section\n" +
		"S 2 900000 section\n" +
		"E 2 880000 section\n" +
		"M 3 99 chapter\n" +
		"S 3 800000 chapter\n"
	if err := os.WriteFile(posPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	diags := checkSectionLinebreak(dir)
	if len(diags) != 1 {
		t.Fatalf("want 1 diag, got %d: %+v", len(diags), diags)
	}
	d := diags[0]
	if d.Line != 42 {
		t.Errorf("line: want 42 got %d", d.Line)
	}
	if d.File != "main.tex" {
		t.Errorf("file: want main.tex got %s", d.File)
	}
	if d.Message != "section spans multiple PDF lines" {
		t.Errorf("msg: %q", d.Message)
	}
}

func TestSectionLinebreakNoFile(t *testing.T) {
	dir := t.TempDir()
	diags := checkSectionLinebreak(dir)
	if diags != nil {
		t.Errorf("want nil, got %v", diags)
	}
}
