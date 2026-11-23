package pedantic

import (
	"reflect"
	"testing"
)

func TestCiteTie_Detector(t *testing.T) {
	lines := []string{
		`By~\cite{a}, it holds.`,               // ok
		`\cite{a} implies the claim.`,          // ok: first thing on line
		`    \citep{a} with indent.`,           // ok: first content
		`By \cite{a}, it holds.`,               // flag
		`Using (\cite{a}) and {\citet{b}}.`,    // ok
		`See \cite{a}--\cite{b}.`,              // flag first only
		`Hence,\parencite{a} follows.`,         // flag (glued)
		`$x = y \cite{a}$`,                     // ok: math
		`% comment \cite{a}`,                   // ok: comment (runner strips)
		`Refs \citeyear{a} are another macro.`, // ok
		`Thus \Citep{a} shows this.`,           // flag: capitalized variant
	}
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = l
		if i == 8 {
			stripped[i] = ""
		}
	}
	diags := checkCiteTie(nil, "t.tex", stripped)
	var got []int
	for _, d := range diags {
		got = append(got, d.Line)
	}
	want := []int{4, 6, 7, 11}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %v, want %v (%+v)", got, want, diags)
	}
}

func TestCiteTie_Fixer(t *testing.T) {
	in := []string{
		`By \cite{a}, it holds.`,
		`By  \cite{a} and \citep{b}. % see \cite{c}`,
		`Hence,\parencite{a} follows.`,
		`By ~ \cite{a}.`,
		`\cite{a} stays.`,
		`By~\cite{a} stays.`,
	}
	want := []string{
		`By~\cite{a}, it holds.`,
		`By~\cite{a} and~\citep{b}. % see \cite{c}`,
		`Hence,~\parencite{a} follows.`,
		`By ~\cite{a}.`,
		`\cite{a} stays.`,
		`By~\cite{a} stays.`,
	}
	out, changed := fixCiteTie(nil, "t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changes")
	}
	if !reflect.DeepEqual(out, want) {
		for i := range want {
			if out[i] != want[i] {
				t.Errorf("line %d: got %q, want %q", i+1, out[i], want[i])
			}
		}
	}
	if _, changed := fixCiteTie(nil, "t.tex", append([]string(nil), want...)); changed {
		t.Error("fixer not idempotent")
	}
}
