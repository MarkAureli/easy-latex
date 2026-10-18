package lsp

import (
	"reflect"
	"testing"
)

func TestCompleteRef(t *testing.T) {
	doc := "\\label{sec:intro}\n\\label{equation:a}\n% \\label{equation:commented}\n\\label{fig:x} \\label{equation:b}\n"
	tests := []struct {
		name string
		line string
		want []string
	}{
		{"cref all", `\cref{`, []string{"equation:a", "equation:b", "fig:x", "sec:intro"}},
		{"autoref prefix", `\autoref{fig`, []string{"fig:x"}},
		{"eqref filters", `\eqref{`, []string{"equation:a", "equation:b"}},
		{"eqref after comma", `\eqref{equation:a,equation:b`, []string{"equation:b"}},
		{"cref optional arg", `\cref*[x]{s`, []string{"sec:intro"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &server{docs: map[string]string{"test.tex": doc + tt.line}}
			got := []string{}
			for _, it := range s.complete(completionParams{
				TextDocument: textDocumentIdentifier{URI: "test.tex"},
				Position:     position{Line: 4, Character: len(tt.line)},
			}) {
				got = append(got, it.Label)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCiteCloseBrace(t *testing.T) {
	tests := []struct {
		name    string
		enabled []string
		text    string
		char    int
		want    string
	}{
		{"enabled", []string{"no-block-citations"}, `\cite{Sm`, 8, "Smith}"},
		{"brace already present", []string{"no-block-citations"}, `\cite{Sm}`, 8, ""},
		{"disabled", nil, `\cite{Sm`, 8, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &server{
				docs:          map[string]string{"t.tex": tt.text},
				items:         []completionItem{{Label: "Smith"}},
				enabledChecks: tt.enabled,
			}
			got := s.complete(completionParams{
				TextDocument: textDocumentIdentifier{URI: "t.tex"},
				Position:     position{Character: tt.char},
			})
			if len(got) != 1 || got[0].InsertText != tt.want {
				t.Fatalf("got %+v, want InsertText %q", got, tt.want)
			}
		})
	}
}
