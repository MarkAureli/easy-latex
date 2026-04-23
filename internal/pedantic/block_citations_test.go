package pedantic

import (
	"testing"
)

func TestCheckBlockCitations(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		count int // expected diagnostics
	}{
		{"single key ok", `See \cite{Smith2020} for details.`, 0},
		{"multi key", `See \cite{Smith2020,Jones2021} for details.`, 1},
		{"adjacent same cmd", `See \cite{Smith2020}\cite{Jones2021}.`, 1},
		{"adjacent space", `See \cite{Smith2020} \cite{Jones2021}.`, 1},
		{"adjacent tilde", `See \cite{Smith2020}~\cite{Jones2021}.`, 1},
		{"adjacent mixed cmds", `See \cite{Smith2020}\citet{Jones2021}.`, 1},
		{"adjacent parencite textcite", `See \parencite{A}~\textcite{B}.`, 1},
		{"separated by text ok", `See \cite{Smith2020} and also \cite{Jones2021}.`, 0},
		{"optional args ok", `See \cite[p.~42]{Smith2020}.`, 0},
		{"starred ok", `See \cite*{Smith2020}.`, 0},
		{"multi key and adjacent", `See \cite{A,B}\cite{C}.`, 2},
		{"three adjacent", `\cite{A}\cite{B}\cite{C}`, 2},
		{"no cite", `Just some text.`, 0},
		{"Capitalized Cite", `See \Cite{A,B}.`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := checkBlockCitations("test.tex", []string{tt.line})
			if len(diags) != tt.count {
				t.Errorf("got %d diagnostics, want %d\ndiags: %v", len(diags), tt.count, diags)
			}
		})
	}
}
