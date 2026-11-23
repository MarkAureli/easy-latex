package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func realPath(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}
	return p
}

func TestFindProjectRoot_InProjectDir(t *testing.T) {
	dir := realPath(t, t.TempDir())
	os.MkdirAll(filepath.Join(dir, ".el"), 0755)
	chdir(t, dir)

	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != dir {
		t.Errorf("got %q, want %q", root, dir)
	}
}

func TestFindProjectRoot_InSubdir(t *testing.T) {
	dir := realPath(t, t.TempDir())
	os.MkdirAll(filepath.Join(dir, ".el"), 0755)
	sub := filepath.Join(dir, "src", "deep")
	os.MkdirAll(sub, 0755)
	chdir(t, sub)

	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != dir {
		t.Errorf("got %q, want %q", root, dir)
	}
}

func TestFindProjectRoot_NoProject(t *testing.T) {
	chdir(t, t.TempDir())

	_, err := findProjectRoot()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "fatal: not an el project (or any of the parent directories): .el"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("got %q, want it to contain %q", err.Error(), want)
	}
}
