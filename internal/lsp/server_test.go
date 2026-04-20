package lsp

import "testing"

func TestDetectCitePrefix(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		char   int
		prefix string
		ok     bool
	}{
		// standard
		{"cite", `\cite{foo`, 9, "foo", true},
		{"cite empty", `\cite{`, 6, "", true},
		{"cite multi", `\cite{foo,bar`, 13, "bar", true},
		// natbib basic
		{"citet", `\citet{key`, 10, "key", true},
		{"citep", `\citep{key`, 10, "key", true},
		{"citealt", `\citealt{key`, 12, "key", true},
		{"citealp", `\citealp{key`, 12, "key", true},
		{"citeauthor", `\citeauthor{key`, 15, "key", true},
		{"citeyear", `\citeyear{key`, 13, "key", true},
		{"citeyearpar", `\citeyearpar{key`, 16, "key", true},
		{"citenum", `\citenum{key`, 12, "key", true},
		// capitalised
		{"Citet", `\Citet{key`, 10, "key", true},
		{"Citep", `\Citep{key`, 10, "key", true},
		{"Citealt", `\Citealt{key`, 12, "key", true},
		{"Citealp", `\Citealp{key`, 12, "key", true},
		{"Citeauthor", `\Citeauthor{key`, 15, "key", true},
		// starred
		{"citet*", `\citet*{key`, 11, "key", true},
		{"citep*", `\citep*{key`, 11, "key", true},
		{"citealt*", `\citealt*{key`, 13, "key", true},
		{"Citet*", `\Citet*{key`, 11, "key", true},
		// optional args
		{"citep opt", `\citep[see][p.1]{key`, 20, "key", true},
		{"citet opt", `\citet[e.g.]{key`, 16, "key", true},
		{"Citep opt", `\Citep[see][]{key`, 17, "key", true},
		// no match
		{"no cite", `hello world`, 11, "", false},
		{"closed brace", `\cite{key}`, 10, "", false},
		{"citetext", `\citetext{foo`, 13, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, ok := detectCitePrefix(tt.line, tt.char)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if prefix != tt.prefix {
				t.Fatalf("prefix = %q, want %q", prefix, tt.prefix)
			}
		})
	}
}
