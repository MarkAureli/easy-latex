package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func TestRefreshEnvTable(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".el"), 0755); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(root, "main.tex")
	table := filepath.Join(root, ".el", "envs.json")
	// tabular has a usage-learned count; lemma was a theorem before.
	if err := os.WriteFile(table, []byte(`{"tabular":{"args":1,"theorem":false},"lemma":{"args":0,"theorem":true}}`), 0644); err != nil {
		t.Fatal(err)
	}
	src := "\\newcommand{\\abs}[1]{#1}\n\\newtheorem{claim}{Claim}\n\\newenvironment{box}[1]{}{}\n\\begin{tabular}{c}{d}\\end{tabular}\n"
	if err := os.WriteFile(main, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	refreshEnvTable("file://" + main)

	got := texscan.LoadTable[texscan.EnvInfo](table)
	if !got["claim"].Theorem {
		t.Errorf("claim should be a theorem: %v", got)
	}
	if got["lemma"].Theorem {
		t.Errorf("lemma no longer declared: %v", got)
	}
	if got["box"].Args != 1 {
		t.Errorf("box args: %v", got["box"])
	}
	if cmds := texscan.LoadTable[int](filepath.Join(root, ".el", "cmds.json")); cmds["abs"] != 1 {
		t.Errorf("declared command count: %v", cmds)
	}
	if got["tabular"].Args != 1 {
		t.Errorf("usage-derived tabular count must stay untouched: %v", got["tabular"])
	}
}

func TestRefreshEnvTableOutsideProject(t *testing.T) {
	dir := t.TempDir()
	refreshEnvTable("file://" + filepath.Join(dir, "a.tex"))
	if _, err := os.Stat(filepath.Join(dir, ".el")); err == nil {
		t.Fatal("must not create .el outside a project")
	}
}
