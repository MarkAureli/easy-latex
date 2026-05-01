package pedantic

import (
	"testing"
)

func TestCheckMathBareWord(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		count int
	}{
		// single letters are fine
		{"single letter inline", `$x = 5$`, 0},
		{"single letter subscript", `$x_i = 5$`, 0},
		// commands are fine
		{"command", `$\pauli = 5$`, 0},
		{"command with arg", `$\alpha + \beta = 1$`, 0},
		// \text family exempt
		{"text", `$\text{Pauli} = 5$`, 0},
		{"textbf", `$\textbf{abc} = 5$`, 0},
		{"textrm", `$\textrm{abc} = 5$`, 0},
		{"textit", `$\textit{abc} = 5$`, 0},
		{"textsf", `$\textsf{abc} = 5$`, 0},
		{"texttt", `$\texttt{abc} = 5$`, 0},
		// math font commands exempt
		{"mathrm", `$\mathrm{abc} = 5$`, 0},
		{"mathsf", `$\mathsf{abc} = 5$`, 0},
		{"mathtt", `$\mathtt{abc} = 5$`, 0},
		{"mathit", `$\mathit{abc} = 5$`, 0},
		// explicit text boxes exempt
		{"mbox", `$\mbox{text} = 5$`, 0},
		{"hbox", `$\hbox{text} = 5$`, 0},
		// bare words flagged
		{"bare word inline", `$pauli = 5$`, 1},
		{"bare word two letters", `$ab = 5$`, 1},
		{"bare word in display", `\[pauli = 5\]`, 1},
		// bare word in braces (subscript etc.) flagged
		{"bare word in braces", `$e^{ij}$`, 1},
		// mixed: text-wrapped and bare on same line
		{"text ok bare flagged", `$\text{Pauli} + pauli = 5$`, 1},
		// text outside math not flagged
		{"text outside math", `Let pauli be defined as follows.`, 0},
		// no bare word in math with numbers breaking runs
		{"letter-number-letter", `$x1y = 5$`, 0},
		// display math env
		{"display math env", `\begin{equation}pauli = 5\end{equation}`, 1},
		// nested braces inside text cmd
		{"text nested braces", `$\text{a{b}c} = 5$`, 0},
		// intertext exempt
		{"intertext", `\intertext{some text here}`, 0},
		// label argument exempt
		{"label", `$x = 5 \label{eq:pauli}$`, 0},
		{"label bare name", `$x = 5 \label{myeq}$`, 0},
		// begin/end env names exempt
		{"begin env name", `\begin{equation}x = 5\end{equation}`, 0},
		{"begin cases inside math", `\begin{align}\begin{cases}x\end{cases}\end{align}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := checkMathBareWord("test.tex", []string{tt.line})
			if len(diags) != tt.count {
				t.Errorf("got %d diagnostics, want %d\nline: %s\ndiags: %v",
					len(diags), tt.count, tt.line, diags)
			}
		})
	}
}
