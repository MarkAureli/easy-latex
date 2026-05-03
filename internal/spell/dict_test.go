package spell

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeDicts_DedupsAndSorts(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("# header\nfoo\nbar\n\nbaz\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("bar\nqux\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "personal.dic")
	n, err := MergeDicts(out, a, b, filepath.Join(dir, "missing.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("want 4 unique words, got %d", n)
	}
	f, _ := os.Open(out)
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan() // count line
	got := []string{}
	for sc.Scan() {
		got = append(got, sc.Text())
	}
	want := "bar baz foo qux"
	if strings.Join(got, " ") != want {
		t.Errorf("want %q got %q", want, strings.Join(got, " "))
	}
}
