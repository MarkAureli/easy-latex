package pedantic

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// TestChecksNoPanicOnRandomInput feeds random LaTeX-ish token soup through
// every registered check and fixer, failing on any panic.
func TestChecksNoPanicOnRandomInput(t *testing.T) {
	toks := []string{`\cite`, `\citep`, `\eqref`, `\input`, `\usepackage`, `\begin`, `\end`, `\section`, `\label`, `\ref`, `\[`, `\]`, `\\`, `$`, `$$`, `{`, `}`, `[`, `]`, `(`, `)`, `a`, `bc`, `1`, `-`, `--`, `---`, ` `, `  `, "\t", `~`, `%`, `.`, `?`, `,`, `=`, `&`, `x`, `\textbf`, `\mathrm`, `\text`, `equation`, `align`, `verbatim`, `\verb|`, `|`, `\item`, `\par`, `\cline`, `\documentclass`, `\footnote`, `\caption`, `\hline`, `\quad`, `\operatorname`, `\left`, `\right`, `\%`, `\$`, `\{`, `\}`, `\`, `*`, `'`, `\newcommand`, `\def`, `\iffalse`, `\fi`, `\begin{equation}`, `\end{equation}`, `\begin{verbatim}`, `\end{verbatim}`, `\begin{itemize}`, `\end{itemize}`, `ä`, `é`, `–`, `−`}
	rng := rand.New(rand.NewSource(1))
	seen := map[string]bool{}
	for it := 0; it < 20000; it++ {
		nl := 1 + rng.Intn(4)
		lines := make([]string, nl)
		for i := range lines {
			var b strings.Builder
			for k := rng.Intn(14); k > 0; k-- {
				b.WriteString(toks[rng.Intn(len(toks))])
			}
			lines[i] = b.String()
		}
		for _, name := range AllNames() {
			c, _ := Get(name)
			func() {
				defer func() {
					if r := recover(); r != nil {
						key := fmt.Sprintf("%s %v", name, r)
						if !seen[key] {
							seen[key] = true
							t.Errorf("%s panic: %v\n  input: %q", name, r, lines)
						}
					}
				}()
				cp := append([]string(nil), lines...)
				if c.Source != nil {
					c.Source("t.tex", cp)
				}
				if c.Fix != nil {
					cp = append([]string(nil), lines...)
					out, _ := c.Fix("t.tex", cp)
					// idempotence/second-pass panic check
					c.Fix("t.tex", append([]string(nil), out...))
				}
				if c.ProjectSource != nil {
					c.ProjectSource(map[string][]string{"t.tex": lines})
				}
			}()
		}
	}
}
