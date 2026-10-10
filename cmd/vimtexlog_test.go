package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
)

func TestAppendDiagsToLog(t *testing.T) {
	p := filepath.Join(t.TempDir(), "main.log")
	if err := os.WriteFile(p, []byte("orig\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ped := []pedantic.Diagnostic{{File: "a.tex", Line: 3, Message: "bad (col 2)"}}
	spell := []pedantic.Diagnostic{{File: "a.tex", Line: 4, Message: "misspelled"}}
	if err := appendDiagsToLog(p, ped, spell); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	for _, want := range []string{"orig\n", "a.tex:3: [el] bad (col 2)\n", "a.tex:4: [el:spell] misspelled\n"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}
