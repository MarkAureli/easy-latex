package spell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func TestLoadIgnoreMacros_GlobalAndLocal(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "g.txt")
	local := filepath.Join(dir, "l.txt")
	if err := os.WriteFile(global, []byte("# comment\nmycustom\n!legacy\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("\nlocalonly\n"), 0644); err != nil {
		t.Fatal(err)
	}
	set := LoadIgnoreMacros(global, local)
	if !set["mycustom"] || !set["localonly"] {
		t.Error("entries missing")
	}
	if set["!legacy"] || set["legacy"] {
		t.Error("legacy negation line must be dropped")
	}
	if set["# comment"] {
		t.Error("comment loaded")
	}
}

func TestLoadIgnoreMacros_MissingFilesNoop(t *testing.T) {
	if set := LoadIgnoreMacros("/nonexistent/a.txt", "/nonexistent/b.txt"); len(set) != 0 {
		t.Errorf("want empty, got %v", set)
	}
}

func TestTextMacros(t *testing.T) {
	got := TextMacros([]string{"todo", "norm", "emph"}, map[string]bool{"norm": true, "section": true})
	if _, ok := got["todo"]; !ok {
		t.Error("defined macro not whitelisted")
	}
	if _, ok := got["norm"]; ok {
		t.Error("ignored defined macro still whitelisted")
	}
	if _, ok := got["section"]; ok {
		t.Error("ignored built-in still whitelisted")
	}
	if got["textcolor"] != 1 {
		t.Errorf("textcolor skip = %d, want 1", got["textcolor"])
	}
	if got["emph"] != texscan.DefaultTextMacros["emph"] {
		t.Error("defined macro must not override built-in skip count")
	}
}
