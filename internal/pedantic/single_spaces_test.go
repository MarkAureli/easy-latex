package pedantic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSingleSpaces_Detector(t *testing.T) {
	lines := []string{
		"Hello  world.",         // 1: violation
		"Hello world.",          // 2: ok
		"  indented",            // 3: leading whitespace ignored
		"Hello   world  foo.",   // 4: violation (first run reported)
		"",                      // 5: empty
		"\t\tindented with tab", // 6: leading tab ignored
	}
	diags := checkSingleSpaces("t.tex", lines)
	if len(diags) != 2 {
		t.Fatalf("got %d diags, want 2: %v", len(diags), diags)
	}
	if diags[0].Line != 1 || diags[1].Line != 4 {
		t.Errorf("diag lines = %d, %d; want 1, 4", diags[0].Line, diags[1].Line)
	}
}

func TestSingleSpaces_Fixer(t *testing.T) {
	in := []string{
		"Hello  world.",
		"  indented  body",
		"no change here",
		"trailing  %  comment  preserved",
		"foo   bar   baz",
	}
	want := []string{
		"Hello world.",
		"  indented body",
		"no change here",
		"trailing %  comment  preserved",
		"foo bar baz",
	}
	out, changed := fixSingleSpaces("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changed=true")
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, out[i], want[i])
		}
	}
}

func TestSingleSpaces_FixerNoChange(t *testing.T) {
	in := []string{"clean line", "  indented", "% comment  only"}
	_, changed := fixSingleSpaces("t.tex", append([]string(nil), in...))
	if changed {
		t.Error("expected changed=false for clean input (comment-only spaces ignored)")
	}
}

func TestRunSourceFixes_SingleSpaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.tex")
	content := "Hello  world.\nclean line\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	modified, err := RunSourceFixes([]string{"single-spaces"}, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 1 || modified[0] != path {
		t.Errorf("modified = %v, want [%s]", modified, path)
	}
	got, _ := os.ReadFile(path)
	want := "Hello world.\nclean line\n"
	if string(got) != want {
		t.Errorf("file content = %q, want %q", got, want)
	}
}

func TestRunSourceFixes_NoOpWhenClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clean.tex")
	if err := os.WriteFile(path, []byte("clean line\n"), 0644); err != nil {
		t.Fatal(err)
	}
	modified, err := RunSourceFixes([]string{"single-spaces"}, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if len(modified) != 0 {
		t.Errorf("modified = %v, want empty", modified)
	}
}
