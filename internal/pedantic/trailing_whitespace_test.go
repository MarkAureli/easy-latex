package pedantic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrailingWhitespace_Detector(t *testing.T) {
	lines := []string{
		"clean",                 // 1: no trailing
		"trailing spaces   ",    // 2: trailing spaces
		"trailing tabs\t\t",     // 3: trailing tabs
		"mixed \t \t",           // 4: mixed
		"",                      // 5: empty
		"   ",                   // 6: all-whitespace
		"code   % comment",      // 7: pre-comment WS, no real trailing
		"code % comment   ",     // 8: trailing WS in comment tail
		"% bare comment   ",     // 9: trailing in pure-comment line
		"x",                     // 10: clean
	}
	diags := checkTrailingWhitespace("t.tex", lines)
	wantLines := []int{2, 3, 4, 6, 8, 9}
	if len(diags) != len(wantLines) {
		t.Fatalf("got %d diags, want %d: %v", len(diags), len(wantLines), diags)
	}
	for i, d := range diags {
		if d.Line != wantLines[i] {
			t.Errorf("diag[%d].Line = %d, want %d", i, d.Line, wantLines[i])
		}
	}
	if diags[0].Message != "trailing whitespace at column 16" {
		t.Errorf("col message: got %q", diags[0].Message)
	}
}

func TestTrailingWhitespace_PreCommentAlignmentNotFlagged(t *testing.T) {
	lines := []string{"code   % comment"}
	if diags := checkTrailingWhitespace("t.tex", lines); len(diags) != 0 {
		t.Errorf("pre-comment alignment must not be flagged: %v", diags)
	}
}

func TestTrailingWhitespace_Fixer(t *testing.T) {
	in := []string{
		"clean",
		"trailing   ",
		"tabs\t\t",
		"mixed \t",
		"code   % comment",      // pre-comment WS preserved
		"code % comment   ",     // strip trailing in comment tail
		"   ",                   // all-WS → empty
		"",
	}
	want := []string{
		"clean",
		"trailing",
		"tabs",
		"mixed",
		"code   % comment",
		"code % comment",
		"",
		"",
	}
	out, changed := fixTrailingWhitespace("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changed=true")
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, out[i], want[i])
		}
	}
}

func TestTrailingWhitespace_FixerNoChange(t *testing.T) {
	in := []string{"a", "b   c", "code   % comment", ""}
	_, changed := fixTrailingWhitespace("t.tex", append([]string(nil), in...))
	if changed {
		t.Error("expected changed=false")
	}
}

func TestRunSourceFixes_TrailingWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.tex")
	content := "clean\ntrail   \ncode % foo   \n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	modified, err := RunSourceFixes([]string{"no-trailing-whitespace"}, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 1 || modified[0] != path {
		t.Errorf("modified = %v, want [%s]", modified, path)
	}
	got, _ := os.ReadFile(path)
	want := "clean\ntrail\ncode % foo\n"
	if string(got) != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}

func TestRunSourceChecks_TrailingWhitespace_RawLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.tex")
	// pre-comment WS must NOT trigger; trailing in comment tail MUST trigger.
	content := "code   % comment\ncode % comment   \n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	diags := RunSourceChecks([]string{"no-trailing-whitespace"}, []string{path})
	if len(diags) != 1 {
		t.Fatalf("got %d diags, want 1: %v", len(diags), diags)
	}
	if diags[0].Line != 2 {
		t.Errorf("diag.Line = %d, want 2", diags[0].Line)
	}
}
