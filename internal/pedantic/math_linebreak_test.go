package pedantic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMathPosLine(t *testing.T) {
	tests := []struct {
		input   string
		wantID  int
		wantY   int
		wantLn  int
		wantK   string
		wantErr bool
	}{
		{"S 1 43234099 42", 1, 43234099, 42, "S", false},
		{"E 3 40874803 42", 3, 40874803, 42, "E", false},
		{"S 26 31570375 163", 26, 31570375, 163, "S", false},
		{"", 0, 0, 0, "", true},
		{"X 1 2 3", 0, 0, 0, "", true},
		{"S 1 2", 0, 0, 0, "", true},
		{"S abc 2 3", 0, 0, 0, "", true},
	}
	for _, tt := range tests {
		e, kind, err := parseMathPosLine(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseMathPosLine(%q): want error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseMathPosLine(%q): %v", tt.input, err)
			continue
		}
		if kind != tt.wantK {
			t.Errorf("parseMathPosLine(%q): kind = %q, want %q", tt.input, kind, tt.wantK)
		}
		if e.ID != tt.wantID || e.YPos != tt.wantY || e.Line != tt.wantLn {
			t.Errorf("parseMathPosLine(%q): got {%d %d %d}, want {%d %d %d}",
				tt.input, e.ID, e.YPos, e.Line, tt.wantID, tt.wantY, tt.wantLn)
		}
	}
}

func TestParseMathPos(t *testing.T) {
	content := `S 1 43234099 10
E 1 43234099 10
S 2 41661235 12
E 2 40874803 12
S 3 40874803 12
E 3 40874803 12
`
	dir := t.TempDir()
	path := filepath.Join(dir, "main.mathpos")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	starts, ends, err := parseMathPos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(starts) != 3 {
		t.Fatalf("starts: got %d entries, want 3", len(starts))
	}
	if len(ends) != 3 {
		t.Fatalf("ends: got %d entries, want 3", len(ends))
	}

	// ID 1: same y — no linebreak
	if starts[1].YPos != ends[1].YPos {
		t.Error("ID 1: expected same y-positions")
	}
	// ID 2: different y — linebreak
	if starts[2].YPos == ends[2].YPos {
		t.Error("ID 2: expected different y-positions")
	}
	// ID 3: same y — no linebreak
	if starts[3].YPos != ends[3].YPos {
		t.Error("ID 3: expected same y-positions")
	}
}

func TestCheckMathLinebreak(t *testing.T) {
	dir := t.TempDir()

	// Write .mathpos file with one linebreak violation on line 5
	// and one non-violation on line 3.
	mathpos := `S 1 43234099 3
E 1 43234099 3
S 2 41661235 5
E 2 40874803 5
`
	if err := os.WriteFile(filepath.Join(dir, "test.mathpos"), []byte(mathpos), 0644); err != nil {
		t.Fatal(err)
	}

	// Write corresponding .tex file so line validation passes.
	tex := `\documentclass{article}
\begin{document}
Short $x = 1$ math.

Long inline math $a + b + c + d + e$ that breaks.
\end{document}
`
	if err := os.WriteFile(filepath.Join(dir, "test.tex"), []byte(tex), 0644); err != nil {
		t.Fatal(err)
	}

	// checkMathLinebreak reads from CWD-relative paths, so we chdir.
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	diags := checkMathLinebreak(dir)
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	d := diags[0]
	if d.Line != 5 {
		t.Errorf("line = %d, want 5", d.Line)
	}
	if d.File != "test.tex" {
		t.Errorf("file = %q, want %q", d.File, "test.tex")
	}
}

func TestCheckMathLinebreak_FiltersFalsePositives(t *testing.T) {
	dir := t.TempDir()

	// Linebreak on source line 3 which has NO inline math → should be filtered.
	mathpos := `S 1 43234099 3
E 1 40874803 3
`
	if err := os.WriteFile(filepath.Join(dir, "test.mathpos"), []byte(mathpos), 0644); err != nil {
		t.Fatal(err)
	}

	tex := `\documentclass{article}
\begin{document}
No math on this line at all.
\end{document}
`
	if err := os.WriteFile(filepath.Join(dir, "test.tex"), []byte(tex), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	diags := checkMathLinebreak(dir)
	if len(diags) != 0 {
		t.Errorf("got %d diagnostics, want 0 (false positive should be filtered)", len(diags))
	}
}

func TestCheckMathLinebreak_NoFile(t *testing.T) {
	dir := t.TempDir()
	diags := checkMathLinebreak(dir)
	if len(diags) != 0 {
		t.Errorf("got %d diagnostics with no mathpos file, want 0", len(diags))
	}
}

func TestLineHasInlineMath(t *testing.T) {
	lines := []string{
		`No math here.`,
		`Some $x + y$ inline math.`,
		`Using \(a + b\) delimiters.`,
		`Just text with percent \% sign.`,
	}
	tests := []struct {
		lineNo int
		want   bool
	}{
		{1, false},
		{2, true},
		{3, true},
		{4, false},
		{0, false},  // out of bounds
		{5, false},  // out of bounds
	}
	for _, tt := range tests {
		got := lineHasInlineMath(lines, tt.lineNo)
		if got != tt.want {
			t.Errorf("lineHasInlineMath(lines, %d) = %v, want %v", tt.lineNo, got, tt.want)
		}
	}
}
