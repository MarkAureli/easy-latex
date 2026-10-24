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
	for _, want := range []string{"orig\n", "Package el Warning: a.tex: [el] bad (col 2) on input line 3.\n", "Package el Warning: a.tex: [el:spell] misspelled on input line 4.\n"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestAppendDiagsToLogLinelessVbox(t *testing.T) {
	p := filepath.Join(t.TempDir(), "main.log")
	in := "Underfull \\vbox (badness 10000) has occurred while \\output is active []\n"
	if err := os.WriteFile(p, []byte(in), 0644); err != nil {
		t.Fatal(err)
	}
	if err := appendDiagsToLog(p, nil, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	want := "Package el Warning: [vbox] Underfull \\vbox (badness 10000) has occurred while \\output is active on input line 1.\n"
	if !strings.Contains(string(got), want) {
		t.Errorf("missing %q in %q", want, got)
	}
}

func TestAppendDiagsToLogLinelessHbox(t *testing.T) {
	p := filepath.Join(t.TempDir(), "main.log")
	in := "Overfull \\hbox (12.0pt too wide) has occurred while \\output is active []\n"
	if err := os.WriteFile(p, []byte(in), 0644); err != nil {
		t.Fatal(err)
	}
	if err := appendDiagsToLog(p, nil, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	want := "Package el Warning: [hbox] Overfull \\hbox (12.0pt too wide) has occurred while \\output is active on input line 1.\n"
	if !strings.Contains(string(got), want) {
		t.Errorf("missing %q in %q", want, got)
	}
}
