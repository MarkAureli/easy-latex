package pedantic

import (
	"reflect"
	"testing"
)

func TestEqrefTie_Detector(t *testing.T) {
	lines := []string{
		`By~\eqref{eq:a}, it holds.`,               // ok
		`\eqref{eq:a} implies the claim.`,          // ok: first thing on line
		`    \eqref{eq:a} with indent.`,            // ok: first content
		`By \eqref{eq:a}, it holds.`,               // flag
		`Using (\eqref{eq:a}) and {\eqref{eq:b}}.`, // ok
		`See \eqref{eq:a}--\eqref{eq:b}.`,          // flag first only
		`Hence,\eqref{eq:a} follows.`,              // flag (glued)
		`$x = y \eqref{eq:a}$`,                     // ok: math
		`% comment \eqref{eq:a}`,                   // ok: comment (runner strips)
		`Refs \eqrefs{a} are another macro.`,       // ok
	}
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = l
		if i == 8 {
			stripped[i] = ""
		}
	}
	diags := checkEqrefTie(nil, "t.tex", stripped)
	var got []int
	for _, d := range diags {
		got = append(got, d.Line)
	}
	want := []int{4, 6, 7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %v, want %v (%+v)", got, want, diags)
	}
}

func TestEqrefTie_Fixer(t *testing.T) {
	in := []string{
		`By \eqref{eq:a}, it holds.`,
		`By  \eqref{a} and \eqref{b}. % see \eqref{c}`,
		`Hence,\eqref{eq:a} follows.`,
		`By ~ \eqref{eq:a}.`,
		`\eqref{eq:a} stays.`,
		`By~\eqref{eq:a} stays.`,
	}
	want := []string{
		`By~\eqref{eq:a}, it holds.`,
		`By~\eqref{a} and~\eqref{b}. % see \eqref{c}`,
		`Hence,~\eqref{eq:a} follows.`,
		`By ~\eqref{eq:a}.`,
		`\eqref{eq:a} stays.`,
		`By~\eqref{eq:a} stays.`,
	}
	out, changed := fixEqrefTie(nil, "t.tex", append([]string(nil), in...))
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
	if _, changed := fixEqrefTie(nil, "t.tex", append([]string(nil), want...)); changed {
		t.Error("fixer not idempotent")
	}
}
