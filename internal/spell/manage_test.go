package spell

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestResolveTarget(t *testing.T) {
	cases := []struct {
		name                         string
		globalDir, auxDir, lang      string
		isGlobal, isIgnore, isCommon bool
		want                         string
		wantErr                      bool
	}{
		{"local lang", "/g", "/p/.el", "en_US", false, false, false, "/p/.el/spell/en_US.txt", false},
		{"global lang", "/g", "/p/.el", "en_US", true, false, false, "/g/spell/en_US.txt", false},
		{"local common", "/g", "/p/.el", "", false, false, true, "/p/.el/spell/common.txt", false},
		{"global common", "/g", "/p/.el", "", true, false, true, "/g/spell/common.txt", false},
		{"local ignore", "/g", "/p/.el", "", false, true, false, "/p/.el/spell/ignore.txt", false},
		{"global ignore", "/g", "/p/.el", "", true, true, false, "/g/spell/ignore.txt", false},
		{"missing lang errors", "/g", "/p/.el", "", false, false, false, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveTarget(tc.globalDir, tc.auxDir, tc.lang, tc.isGlobal, tc.isIgnore, tc.isCommon)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	if err := ValidateToken(""); err == nil {
		t.Error("empty must error")
	}
	if err := ValidateToken("foo bar"); err == nil {
		t.Error("space must error")
	}
	if err := ValidateToken("foo"); err != nil {
		t.Errorf("plain ok: %v", err)
	}
	if err := ValidateToken("it's"); err != nil {
		t.Errorf("apostrophe ok: %v", err)
	}
	if err := ValidateToken("non-empty"); err != nil {
		t.Errorf("hyphen ok: %v", err)
	}
}

func TestAddTokens_Dict_DedupAndSort(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "lang.txt")
	n, err := AddTokens(p, []string{"foo", "bar", "foo"})
	if err != nil || n != 2 {
		t.Fatalf("first add: n=%d err=%v", n, err)
	}
	n2, _ := AddTokens(p, []string{"baz", "bar"})
	if n2 != 1 {
		t.Errorf("redundant add: n=%d want 1", n2)
	}
	got, _ := ListTokens(p)
	want := []string{"bar", "baz", "foo"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestRemoveTokens_Dict(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "lang.txt")
	_, _ = AddTokens(p, []string{"foo", "bar", "baz"})
	rm, err := RemoveTokens(p, []string{"bar", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if rm != 1 {
		t.Errorf("rm=%d want 1", rm)
	}
	got, _ := ListTokens(p)
	if !slices.Equal(got, []string{"baz", "foo"}) {
		t.Errorf("got %v", got)
	}
}

func TestCompletionCandidates(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ig.txt")
	if err := os.WriteFile(p, []byte("mycustom\nother\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := CompletionCandidates(p)
	if !slices.Equal(got, []string{"mycustom", "other"}) {
		t.Errorf("got %v", got)
	}
}
