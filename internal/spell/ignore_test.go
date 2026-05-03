package spell

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIgnoreMacros_DefaultsApplied(t *testing.T) {
	set := LoadIgnoreMacros()
	if !set["cite"] || !set["ref"] || !set["usepackage"] {
		t.Error("defaults not loaded")
	}
}

func TestLoadIgnoreMacros_AdditiveAndNegation(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "g.txt")
	local := filepath.Join(dir, "l.txt")
	if err := os.WriteFile(global, []byte("# comment\nmycustom\n!cite\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("\nlocalonly\n!ref\n"), 0644); err != nil {
		t.Fatal(err)
	}
	set := LoadIgnoreMacros(global, local)
	if !set["mycustom"] || !set["localonly"] {
		t.Error("additive entries missing")
	}
	if set["cite"] {
		t.Error("global negation of `cite` not applied")
	}
	if set["ref"] {
		t.Error("local negation of `ref` not applied")
	}
	// Defaults not negated still present.
	if !set["usepackage"] {
		t.Error("untouched default removed")
	}
}

func TestLoadIgnoreMacros_MissingFilesNoop(t *testing.T) {
	set := LoadIgnoreMacros("/nonexistent/a.txt", "/nonexistent/b.txt")
	if !set["cite"] {
		t.Error("missing files broke defaults")
	}
}
