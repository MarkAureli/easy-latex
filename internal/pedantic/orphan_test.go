package pedantic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrphanLineDetect(t *testing.T) {
	dir := t.TempDir()
	stem := "main"
	if err := os.WriteFile(filepath.Join(dir, stem+".tex"), []byte("dummy\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// id 1: 2-line paragraph fully on page 1 → ok
	// id 2: 2-line paragraph split page 1 → page 2 → ORPHAN at line 42
	// id 3: 5-line paragraph split page 2 → page 3, linecount=5 → not flagged (ambiguous)
	// id 4: 1-line paragraph spanning P boundary (data oddity) → not flagged
	content := "" +
		"P 1\n" +
		"S 1 5000000 10\n" +
		"E 1 2\n" +
		"S 2 1000000 42\n" +
		"P 2\n" +
		"E 2 2\n" +
		"S 3 25000000 50\n" +
		"P 3\n" +
		"E 3 5\n" +
		"S 4 15000000 99\n" +
		"P 4\n" +
		"E 4 1\n"
	if err := os.WriteFile(filepath.Join(dir, stem+".orphan"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	diags := checkOrphanLine(dir)
	if len(diags) != 1 {
		t.Fatalf("want 1 diag, got %d: %+v", len(diags), diags)
	}
	d := diags[0]
	if d.Line != 42 {
		t.Errorf("line: want 42 got %d", d.Line)
	}
	if d.File != "main.tex" {
		t.Errorf("file: %s", d.File)
	}
	if d.Message != "orphan: 2-line paragraph split across pages 1 and 2" {
		t.Errorf("msg: %q", d.Message)
	}
}

func TestOrphanLineNoFile(t *testing.T) {
	dir := t.TempDir()
	if diags := checkOrphanLine(dir); diags != nil {
		t.Errorf("want nil, got %v", diags)
	}
}
