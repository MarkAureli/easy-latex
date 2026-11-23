package cmdprobe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MarkAureli/easy-latex/internal/texscan"
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
	got, _ := Parse(path)
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
	if args, classes := Parse(filepath.Join(t.TempDir(), "missing")); len(args)+len(classes) != 0 {
		t.Error("missing file should yield empty result")
	}
}

// Math symbol meanings as logged by pdflatex (amsmath) and lualatex
// (unicode-math).
const classLog = `le||\mathchar"3214
cdot||\mathchar"2201
alpha||\mathchar"10B
langle||\protected macro:->\delimiter "426830A 
lvert||\protected macro:->\delimiter 69640972 
vert||\protected macro:->\delimiter "026A30C 
colon||macro:->\protect \colon  
colon|s|macro:->\nobreak \mskip 2mu\mathpunct {}\nonscript \mkern -\thinmuskip {:}
neq||macro:->\protect \neq  
neq|s|macro:->\not =
iff||macro:->\DOTSB \;\Longleftrightarrow \;
coloneqq||\protected macro:->\mathrel {\vcentcolon =}
ule||\protected macro:->\leq 
uin||the character ∈
ulangle||\protected macro:->\Udelimiter 4\symoperators "027E8\scan_stop: 
um||\Umathchar"2"0"002212
sin||\protected macro:->\qopname \relax o{sin}
frac||macro:#1#2->x
`

func TestParseClasses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x"+Ext)
	if err := os.WriteFile(path, []byte(classLog), 0644); err != nil {
		t.Fatal(err)
	}
	_, got := Parse(path)
	want := map[string]texscan.MathClass{
		"le": texscan.MathRel, "cdot": texscan.MathBin, "alpha": texscan.MathOrd,
		"langle": texscan.MathOpen, "lvert": texscan.MathOpen, "vert": texscan.MathOrd,
		"coloneqq": texscan.MathRel, "ule": texscan.MathRel, "uin": texscan.MathRel,
		"ulangle": texscan.MathOpen, "um": texscan.MathBin,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	for _, k := range []string{"colon", "neq", "iff", "sin", "frac"} {
		if c, ok := got[k]; ok {
			t.Errorf("%s should be unresolved, got %q", k, c)
		}
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
