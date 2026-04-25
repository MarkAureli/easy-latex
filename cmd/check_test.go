package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func writeCheckConfig(t *testing.T, dir, main string, checks ...string) {
	t.Helper()
	elDir := filepath.Join(dir, ".el")
	if err := os.MkdirAll(elDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pc := PedanticConfig{Checks: map[string]*bool{}}
	tr := true
	for _, c := range checks {
		pc.Checks[c] = &tr
	}
	cfg := Config{Main: main, Pedantic: pc}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(elDir, "config.json"), data, 0644); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
}

func invokeCheck(t *testing.T, args []string) error {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().Bool("fix", false, "")
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	checkFix, _ = cmd.Flags().GetBool("fix")
	defer func() { checkFix = false }()
	return runCheck(cmd, cmd.Flags().Args())
}

func TestCheck_NoChecksEnabled(t *testing.T) {
	dir := t.TempDir()
	writeCheckConfig(t, dir, "main.tex")
	os.WriteFile(filepath.Join(dir, "main.tex"), []byte("clean\n"), 0644)
	chdir(t, dir)

	if err := invokeCheck(t, nil); err == nil {
		t.Fatal("expected error when no checks enabled")
	}
}

func TestCheck_CleanFile(t *testing.T) {
	dir := t.TempDir()
	writeCheckConfig(t, dir, "main.tex", "single-spaces")
	os.WriteFile(filepath.Join(dir, "main.tex"), []byte("clean line\n"), 0644)
	chdir(t, dir)

	if err := invokeCheck(t, nil); err != nil {
		t.Fatalf("expected nil error for clean file, got: %v", err)
	}
}

func TestCheck_DetectsViolation(t *testing.T) {
	dir := t.TempDir()
	writeCheckConfig(t, dir, "main.tex", "single-spaces")
	os.WriteFile(filepath.Join(dir, "main.tex"), []byte("Hello  world.\n"), 0644)
	chdir(t, dir)

	if err := invokeCheck(t, nil); err == nil {
		t.Fatal("expected error for file with violations")
	}
}

func TestCheck_FixApplies(t *testing.T) {
	dir := t.TempDir()
	writeCheckConfig(t, dir, "main.tex", "single-spaces")
	path := filepath.Join(dir, "main.tex")
	os.WriteFile(path, []byte("Hello  world.\n"), 0644)
	chdir(t, dir)

	if err := invokeCheck(t, []string{"--fix"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "Hello world.\n" {
		t.Errorf("file content = %q, want %q", got, "Hello world.\n")
	}
}

func TestCheck_MissingMain(t *testing.T) {
	dir := t.TempDir()
	writeCheckConfig(t, dir, "missing.tex", "single-spaces")
	chdir(t, dir)

	if err := invokeCheck(t, nil); err == nil {
		t.Fatal("expected error when main file missing")
	}
}
