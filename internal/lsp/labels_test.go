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
