package cmdprobe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Log lines as written by pdflatex (TeX Live 2026, amsmath + hyperref).
const probeLog = `LaTeX||macro:->\protect \LaTeX  
LaTeX|s|macro:->L\kern -.36em{\sbox \z@ T}
binom||macro:->\protect \binom  
binom|s|macro:->\genfrac ()\z@ {}
delim||macro:#1.#2->
efoo||macro:->\__cmd_start_expandable:nNNNNn {m o m}\efoo  \efoo  \efoo code ?{}
efoo|s|macro:#1#2->#1{#2}
frac||macro:->\protect \frac  
frac|s|\long macro:#1#2->{\begingroup #1\endgroup \@@over #2}
hat||macro:->\protect \hat  
hat|s|macro:->\mathaccentV {hat}05E
label||macro:#1->\@bsphack \begingroup
left||\left
nopt||macro:->\@protected@testopt \nopt \\nopt {d}
nopt|b|\long macro:[#1]#2->
oonly||macro:->\@protected@testopt \oonly \\oonly {d}
oonly|b|\long macro:[#1]->
plain||\long macro:#1#2#3->
pl||\protected\long macro:#1->
ref||\protected macro:->\__cmd_start:nNNnnn {s}\ref  \ref code {\__cmd_grab_t:w *}{}{}
ref|s|\relax
ropt||macro:->\protect \ropt  
ropt|s|macro:->\@protected@testopt \ropt  \\ropt  {d}
ropt|bs|\long macro:[#1]#2->
section||macro:->\@startsection {section}{1}{\z@ }{-3.5ex}{2.3ex}{\normalfont }
sqrt||macro:->\protect \sqrt  
sqrt|s|macro:->\@ifnextchar [\@sqrt \sqrtsign 
xfoo||\protected macro:->\__cmd_start:nNNnnn {s m O{x} m}\xfoo  \xfoo code {}{}{}
garbage
`

func TestParse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x"+Ext)
	if err := os.WriteFile(path, []byte(probeLog), 0644); err != nil {
		t.Fatal(err)
	}
	got := Parse(path)
	want := map[string]int{
		"efoo": 2, "frac": 2, "label": 1, "nopt": 1, "plain": 3, "pl": 1,
		"ropt": 1, "xfoo": 2, "xfoo*": 2,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %d, want %d", k, got[k], v)
		}
	}
	// Helpers, branches, delimited parameters and 0 counts abort.
	for _, k := range []string{"LaTeX", "binom", "delim", "hat", "left", "oonly", "ref", "ref*", "section", "sqrt", "efoo*"} {
		if n, ok := got[k]; ok {
			t.Errorf("%s should abort, got %d", k, n)
		}
	}
	if len(Parse(filepath.Join(t.TempDir(), "missing"))) != 0 {
		t.Error("missing file should yield empty result")
	}
}

func TestUndelimitedParams(t *testing.T) {
	for params, want := range map[string]int{"#1": 1, "#1#2#3": 3, "#1.#2": 0, "#1#2.": 1, "#1{": 0, "#1#2{": 1, "[#1]#2": 0, "": 0} {
		if got := undelimitedParams(params); got != want {
			t.Errorf("%q: got %d, want %d", params, got, want)
		}
	}
}

func TestSty(t *testing.T) {
	sty := string(Sty([]string{"frac", "xfoo*", "frac", "@bad", "x1"}))
	for _, want := range []string{`\ProvidesPackage{el-cmdprobe}`, `\elcmdprobe{frac}`, `\elcmdprobe{xfoo}`, `\ifcsname`, `begindocument/end`} {
		if !strings.Contains(sty, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Count(sty, `\elcmdprobe{frac}`) != 1 || strings.Contains(sty, "@bad}") || strings.Contains(sty, "{x1}") {
		t.Error("names must be deduplicated and restricted to letters")
	}
}
