package spell

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireHunspell(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("hunspell"); err != nil {
		t.Skip("hunspell binary not in PATH")
	}
	hs, err := StartHunspell("en_US", "")
	if err != nil {
		t.Skip("hunspell en_US dictionary not installed")
	}
	hs.Close()
}

func TestHunspellAvailable_NoBinary(t *testing.T) {
	t.Setenv("PATH", "")
	var buf bytes.Buffer
	if HunspellAvailable("en_US", &buf) {
		t.Error("expected unavailable with empty PATH")
	}
	if !strings.Contains(buf.String(), "hunspell") {
		t.Errorf("expected warning, got %q", buf.String())
	}
}

func TestHunspell_PipeMode_DetectsTypo(t *testing.T) {
	requireHunspell(t)
	hs, err := StartHunspell("en_US", "")
	if err != nil {
		t.Fatal(err)
	}
	defer hs.Close()
	misses, err := hs.CheckLine("Helo wrld")
	if err != nil {
		t.Fatal(err)
	}
	if len(misses) < 2 {
		t.Errorf("want >=2 misses, got %d: %#v", len(misses), misses)
	}
}

func TestHunspell_PipeMode_PersonalDictAccepts(t *testing.T) {
	requireHunspell(t)
	dir := t.TempDir()
	wordsPath := filepath.Join(dir, "w.txt")
	if err := os.WriteFile(wordsPath, []byte("blarflnerg\n"), 0644); err != nil {
		t.Fatal(err)
	}
	personal := filepath.Join(dir, "p.dic")
	if _, err := MergeDicts(personal, wordsPath); err != nil {
		t.Fatal(err)
	}
	hs, err := StartHunspell("en_US", personal)
	if err != nil {
		t.Fatal(err)
	}
	defer hs.Close()
	misses, err := hs.CheckLine("blarflnerg")
	if err != nil {
		t.Fatal(err)
	}
	if len(misses) != 0 {
		t.Errorf("personal dict ignored: %#v", misses)
	}
}
