// Package cmdprobe instruments a real compile to learn how many mandatory
// arguments commands take once every package and class is loaded. A generated
// .sty logs the \meaning of each candidate command (and of the helpers the
// standard LaTeX wrappers delegate to) to <jobname>.elcmds at the end of
// \begin{document}; Parse resolves those meanings into argument counts.
//
// Only the standard wrappers are followed: plain macros, \DeclareRobustCommand
// (\protect\foo␣), \newcommand with an optional argument (\@testopt /
// \@protected@testopt → \\foo) and ltcmd/xparse document commands (spec read
// from the meaning). Anything else aborts, and so does a count of 0: commands
// that hand their arguments to a helper (\section → \@startsection, \ref with
// spec {s}) show no parameters of their own, so 0 is never trusted.
package cmdprobe

import (
	"bufio"
	"os"
	"sort"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// PkgName is the name (without .sty) of the generated probe package.
const PkgName = "el-cmdprobe"

// Ext is the extension of the side file the probe writes next to the .aux.
const Ext = ".elcmds"

// Variants of a command name whose meanings are logged.
const (
	kindBase      = ""   // \foo
	kindSpace     = "s"  // \foo␣ (robust inner)
	kindBackslash = "b"  // \\foo (optional-argument inner)
	kindBoth      = "bs" // \\foo␣ (robust optional-argument inner)
)

// Sty returns the probe package source for the given command names. Starred
// names are probed through their base name. Hooks need the LaTeX kernel's
// \AddToHook (2020-10+); on older kernels the package does nothing. Every
// lookup is guarded by \ifcsname so unknown names are not defined as \relax.
func Sty(names []string) []byte {
	set := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSuffix(n, "*")
		if isLetters(n) {
			set[n] = true
		}
	}
	sorted := make([]string, 0, len(set))
	for n := range set {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)

	var b strings.Builder
	b.WriteString(`\ProvidesPackage{` + PkgName + `}
\ifdefined\AddToHook
\newwrite\elcmdprobe@file
\def\elcmdprobe@w#1#2{\ifcsname#2\endcsname\immediate\write\elcmdprobe@file{#1|\expandafter\meaning\csname#2\endcsname}\fi}
\def\elcmdprobe#1{%
  \elcmdprobe@w{#1|` + kindBase + `}{#1}%
  \elcmdprobe@w{#1|` + kindSpace + `}{#1\space}%
  \elcmdprobe@w{#1|` + kindBackslash + `}{\@backslashchar#1}%
  \elcmdprobe@w{#1|` + kindBoth + `}{\@backslashchar#1\space}}
\AddToHook{begindocument/end}{%
\begingroup\escapechar=92 \newlinechar=-1
\immediate\openout\elcmdprobe@file=\jobname` + Ext + `\relax
`)
	for _, n := range sorted {
		b.WriteString(`\elcmdprobe{` + n + "}\n")
	}
	b.WriteString("\\immediate\\closeout\\elcmdprobe@file\n\\endgroup}\n\\fi\n")
	return []byte(b.String())
}

// Parse reads a probe log and returns the resolved mandatory-argument count
// per command name (starred names included where the ltcmd spec has a leading
// star) and the math class of every symbol command it can classify (see
// mathClass). Unresolved commands are absent. A missing file yields empty
// results.
func Parse(path string) (map[string]int, map[string]texscan.MathClass) {
	meanings := readMeanings(path)
	return resolveAll(meanings), classifyAll(meanings)
}

// readMeanings reads a probe log into name -> kind -> meaning.
func readMeanings(path string) map[string]map[string]string {
	meanings := map[string]map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return meanings
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), "|")
		if !ok {
			continue
		}
		kind, meaning, ok := strings.Cut(rest, "|")
		if !ok {
			continue
		}
		if meanings[name] == nil {
			meanings[name] = map[string]string{}
		}
		meanings[name][kind] = meaning
	}
	return meanings
}

func resolveAll(meanings map[string]map[string]string) map[string]int {
	out := map[string]int{}
	for name, m := range meanings {
		n, starred := resolve(name, m, kindBase, 0)
		if n <= 0 {
			continue
		}
		out[name] = n
		if starred {
			out[name+"*"] = n
		}
	}
	return out
}

// resolve returns the mandatory-argument count of the meaning m[kind] (0 when
// unresolved) and whether it is an ltcmd command with a leading star argument.
func resolve(name string, m map[string]string, kind string, depth int) (int, bool) {
	meaning, ok := m[kind]
	if !ok || depth > 2 {
		return 0, false
	}
	params, body, ok := splitMacro(meaning)
	if !ok {
		return 0, false
	}
	if params != "" {
		return undelimitedParams(params), false
	}
	body = strings.TrimSpace(body)
	inner := map[string]string{kindBase: kindBackslash, kindSpace: kindBoth}
	switch {
	case kind == kindBase && body == `\protect \`+name:
		return resolve(name, m, kindSpace, depth+1)
	case strings.HasPrefix(body, `\@protected@testopt `), strings.HasPrefix(body, `\@testopt `):
		ik, ok := inner[kind]
		if !ok {
			return 0, false
		}
		p, _, ok := splitMacro(m[ik])
		if !ok || !strings.HasPrefix(p, "[#1]") {
			return 0, false
		}
		return undelimitedParams(renumber(strings.TrimPrefix(p, "[#1]"))), false
	case strings.HasPrefix(body, `\__cmd_start:`), strings.HasPrefix(body, `\__cmd_start_expandable:`):
		spec, ok := firstGroup(body)
		if !ok {
			return 0, false
		}
		spec = strings.TrimSpace(spec)
		return texscan.DocumentSpecMandatory(spec), strings.HasPrefix(spec, "s") || strings.HasPrefix(spec, "t*")
	}
	return 0, false
}

// splitMacro splits a \meaning of a macro into parameter text and body,
// dropping \long/\protected/\outer prefixes. Non-macros report !ok.
func splitMacro(meaning string) (params, body string, ok bool) {
	for {
		trimmed := meaning
		for _, p := range []string{`\long`, `\protected`, `\outer`} {
			trimmed = strings.TrimLeft(strings.TrimPrefix(trimmed, p), " ")
		}
		if trimmed == meaning {
			break
		}
		meaning = trimmed
	}
	rest, ok := strings.CutPrefix(meaning, "macro:")
	if !ok {
		return "", "", false
	}
	params, body, ok = strings.Cut(rest, "->")
	return params, body, ok
}

// undelimitedParams counts the leading undelimited parameters of a macro's
// parameter text: #1#2.#3 has one (#2 is delimited by ".").
func undelimitedParams(params string) int {
	n := 0
	for i := 0; i+1 < len(params) && params[i] == '#' && params[i+1] >= '1' && params[i+1] <= '9'; i += 2 {
		if i+2 < len(params) && params[i+2] != '#' {
			break // delimited by following text (#{ shows as {)
		}
		n++
	}
	return n
}

// renumber shifts #2#3… to #1#2… after the optional [#1] was stripped, so
// undelimitedParams sees a parameter text starting at #1. Only the digits
// matter to it, so a plain shift is enough.
func renumber(params string) string {
	b := []byte(params)
	for i := 0; i+1 < len(b); i++ {
		if b[i] == '#' && b[i+1] >= '2' && b[i+1] <= '9' {
			b[i+1]--
		}
	}
	return string(b)
}

// firstGroup returns the contents of the first balanced {...} group in s.
func firstGroup(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start+1 : i], true
			}
		}
	}
	return "", false
}

func isLetters(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return false
		}
	}
	return true
}
