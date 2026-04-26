package pedantic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNoTabs_Detector(t *testing.T) {
	lines := []string{
		"\thello",                // 1: leading tab
		"hello world",            // 2: clean
		"hello\tworld",           // 3: mid-line tab
		"",                       // 4: empty
		"\\begin{verbatim}",      // 5
		"foo\tbar",               // 6: tab inside verbatim — ignored
		"\\end{verbatim}",        // 7
		"after\tverbatim",        // 8: tab outside again
	}
	diags := checkNoTabs("t.tex", lines)
	if len(diags) != 3 {
		t.Fatalf("got %d diags, want 3: %v", len(diags), diags)
	}
	wantLines := []int{1, 3, 8}
	for i, d := range diags {
		if d.Line != wantLines[i] {
			t.Errorf("diag[%d].Line = %d, want %d", i, d.Line, wantLines[i])
		}
	}
	if diags[1].Message != "tab character at column 6" {
		t.Errorf("got %q", diags[1].Message)
	}
}

func TestNoTabs_Fixer(t *testing.T) {
	in := []string{
		"\thello",
		"hello\tworld",
		"a\tb\tc",
		"clean line",
		"\\begin{verbatim}",
		"foo\tbar",
		"\\end{verbatim}",
		"trail\t% tab\there",
	}
	want := []string{
		"    hello",
		"hello   world",
		"a   b   c",
		"clean line",
		"\\begin{verbatim}",
		"foo\tbar",
		"\\end{verbatim}",
		"trail   % tab   here",
	}
	out, changed := fixNoTabs("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changed=true")
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, out[i], want[i])
		}
	}
}

func TestNoTabs_FixerNoChange(t *testing.T) {
	in := []string{"clean line", "  spaces only", "\\begin{verbatim}", "foo\tbar", "\\end{verbatim}"}
	_, changed := fixNoTabs("t.tex", append([]string(nil), in...))
	if changed {
		t.Error("expected changed=false (only verbatim tab present)")
	}
}

func TestNoTabs_ColumnAware(t *testing.T) {
	in := []string{"ab\tcd"}
	want := []string{"ab  cd"}
	out, _ := fixNoTabs("t.tex", append([]string(nil), in...))
	if out[0] != want[0] {
		t.Errorf("got %q, want %q", out[0], want[0])
	}
}

func TestRunSourceFixes_NoTabs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.tex")
	content := "\thello\nclean\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	modified, err := RunSourceFixes([]string{"no-tabs"}, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 1 || modified[0] != path {
		t.Errorf("modified = %v, want [%s]", modified, path)
	}
	got, _ := os.ReadFile(path)
	want := "    hello\nclean\n"
	if string(got) != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}
