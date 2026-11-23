package lsp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckContextCachesByMtime(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".el"), 0755); err != nil {
		t.Fatal(err)
	}
	tbl := filepath.Join(root, ".el", "cmds.json")
	write := func(s string, mtime time.Time) {
		if err := os.WriteFile(tbl, []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(tbl, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	s := &server{}
	path := filepath.Join(root, "main.tex")
	t0 := time.Now().Add(-time.Hour)

	write(`{"foo": 1}`, t0)
	if n, _ := s.checkContext(path).Args("foo"); n != 1 {
		t.Fatalf("foo: got %d", n)
	}
	if n, _ := s.checkContext(path).Args("frac"); n != 2 {
		t.Fatalf("built-in frac: got %d", n)
	}
	write(`{"foo": 3}`, t0) // same mtime: cached
	if n, _ := s.checkContext(path).Args("foo"); n != 1 {
		t.Fatalf("cached foo: got %d", n)
	}
	write(`{"foo": 3}`, t0.Add(time.Minute))
	if n, _ := s.checkContext(path).Args("foo"); n != 3 {
		t.Fatalf("reloaded foo: got %d", n)
	}
	if n, ok := (&server{}).checkContext(filepath.Join(t.TempDir(), "x.tex")).Args("frac"); !ok || n != 2 {
		t.Fatal("outside a project the built-in table applies")
	}
}
