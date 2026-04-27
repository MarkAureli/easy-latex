package pedantic

import (
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestCheckUnusedLabels(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []string // expected "file:line:name" tokens
	}{
		{
			name: "ref same file",
			files: map[string]string{
				"a.tex": `\label{foo}` + "\n" + `\ref{foo}`,
			},
		},
		{
			name: "unused bare label",
			files: map[string]string{
				"a.tex": `\label{foo}`,
			},
			want: []string{"a.tex:1:foo"},
		},
		{
			name: "ref cross file",
			files: map[string]string{
				"a.tex": `\label{foo}`,
				"b.tex": `\autoref{foo}`,
			},
		},
		{
			name: "cleveref multi-key",
			files: map[string]string{
				"a.tex": `\label{x}` + "\n" + `\label{y}` + "\n" + `\label{z}`,
				"b.tex": `\cref{x, y,z}`,
			},
		},
		{
			name: "hyperref bracket",
			files: map[string]string{
				"a.tex": `\label{foo}` + "\n" + `\hyperref[foo]{see}`,
			},
		},
		{
			name: "ignored prefix theorem",
			files: map[string]string{
				"a.tex": `\label{theorem:big}`,
			},
		},
		{
			name: "ignored prefix section",
			files: map[string]string{
				"a.tex": `\label{section:intro}`,
			},
		},
		{
			name: "non-ignored prefix figure",
			files: map[string]string{
				"a.tex": `\label{figure:plot}`,
			},
			want: []string{"a.tex:1:figure:plot"},
		},
		{
			name: "non-ignored prefix equation",
			files: map[string]string{
				"a.tex": `\label{equation:foo}`,
			},
			want: []string{"a.tex:1:equation:foo"},
		},
		{
			name: "custom prefix flagged",
			files: map[string]string{
				"a.tex": `\label{mythm:foo}`,
			},
			want: []string{"a.tex:1:mythm:foo"},
		},
		{
			name: "verbatim label skipped",
			files: map[string]string{
				"a.tex": `\begin{verbatim}` + "\n" + `\label{ignored}` + "\n" + `\end{verbatim}`,
			},
		},
		{
			name: "math-region label still tracked",
			files: map[string]string{
				"a.tex": `\begin{equation}` + "\n" + `x \label{eqn:foo}` + "\n" + `\end{equation}`,
			},
			want: []string{"a.tex:2:eqn:foo"},
		},
		{
			name: "starred ref",
			files: map[string]string{
				"a.tex": `\label{foo}` + "\n" + `\cref*{foo}`,
			},
		},
		{
			name: "pageref + nameref",
			files: map[string]string{
				"a.tex": `\label{a}\label{b}` + "\n" + `\pageref{a}\nameref{b}`,
			},
		},
		{
			name: "Capitalized Cref",
			files: map[string]string{
				"a.tex": `\label{foo}` + "\n" + `\Cref{foo}`,
			},
		},
		{
			name: "labelcref",
			files: map[string]string{
				"a.tex": `\label{foo}` + "\n" + `\labelcref{foo}`,
			},
		},
		{
			name: "report at def site line",
			files: map[string]string{
				"a.tex": "line1\nline2\n\\label{dead}\nline4",
			},
			want: []string{"a.tex:3:dead"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string][]string{}
			for p, txt := range tt.files {
				files[p] = strings.Split(txt, "\n")
			}
			diags := checkUnusedLabels(files)
			got := make([]string, 0, len(diags))
			for _, d := range diags {
				got = append(got, formatLabelDiag(d))
			}
			sort.Strings(got)
			want := append([]string(nil), tt.want...)
			sort.Strings(want)
			if !equalSlices(got, want) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func formatLabelDiag(d Diagnostic) string {
	const prefix = "unreferenced label: "
	name := strings.TrimPrefix(d.Message, prefix)
	return d.File + ":" + strconv.Itoa(d.Line) + ":" + name
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
