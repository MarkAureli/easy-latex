package envprobe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	log := "itemize|figure,0|figure,1\nminipage|figure,0|figure,0\nfigure|-,0|figure,0\ncenter|-,0|-,0\nwrapfigure|-,0|figure,0\ngarbage\n"
	path := filepath.Join(t.TempDir(), "x.elprobe")
	if err := os.WriteFile(path, []byte(log), 0644); err != nil {
		t.Fatal(err)
	}
	got := Parse(path)
	want := map[string]Result{
		"itemize":    {List: true},
		"minipage":   {},
		"figure":     {Float: true},
		"center":     {},
		"wrapfigure": {Float: true},
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %+v, want %+v", k, got[k], v)
		}
	}
	if len(Parse(filepath.Join(t.TempDir(), "missing"))) != 0 {
		t.Error("missing file should yield empty result")
	}
}

func TestSty(t *testing.T) {
	sty := string(Sty([]string{"lemma", "document", "lemma"}))
	for _, want := range []string{`\elprobe{lemma}`, `\elprobe{figure*}`, `\ProvidesPackage{el-envprobe}`} {
		if !strings.Contains(sty, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(sty, `\elprobe{document}`) || strings.Count(sty, `\elprobe{lemma}`) != 1 {
		t.Error("document must be skipped and names deduplicated")
	}
}
