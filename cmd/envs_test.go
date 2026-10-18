package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readEnvTable(t *testing.T) map[string]int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(auxDir, envsFile))
	if err != nil {
		t.Fatal(err)
	}
	var tbl map[string]int
	if err := json.Unmarshal(data, &tbl); err != nil {
		t.Fatal(err)
	}
	return tbl
}

func TestUpdateEnvTableOverwritesAndKeeps(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.MkdirAll(auxDir, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(s string) {
		if err := os.WriteFile("main.tex", []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}

	write("\\begin{tabular}{c}{d}\n\\end{tabular}\n\\begin{foo}{x}\\end{foo}\n")
	if err := updateEnvTable("main.tex"); err != nil {
		t.Fatal(err)
	}
	if tbl := readEnvTable(t); tbl["tabular"] != 2 || tbl["foo"] != 1 {
		t.Fatalf("first run: %v", tbl)
	}

	// A later occurrence with a single group corrects the signature; foo is gone but kept.
	write("\\begin{tabular}{c}{d}\\end{tabular}\n\\begin{tabular}{c}\\end{tabular}\n")
	if err := updateEnvTable("main.tex"); err != nil {
		t.Fatal(err)
	}
	if tbl := readEnvTable(t); tbl["tabular"] != 1 || tbl["foo"] != 1 {
		t.Fatalf("second run: %v", tbl)
	}
}
