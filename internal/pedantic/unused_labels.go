package pedantic

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

func init() {
	Register(Check{
		Name:          "unused-labels",
		Phase:         PhaseProjectSource,
		ProjectSource: checkUnusedLabels,
	})
}

// reLabelCall matches `\label{name}`.
var reLabelCall = regexp.MustCompile(`\\label\{([^}]+)\}`)

// reRefCall matches reference commands taking a curly-brace name list.
// Covers \ref, \Ref, \eqref, \autoref, \Autoref, \cref, \Cref, \crefrange,
// \Crefrange, \labelcref, \pageref, \Pageref, \nameref, \Nameref, \vref,
// \Vref, \vpageref, \autopageref, plus starred variants and optional args.
// The capture group holds the comma-separated key list.
var reRefCall = regexp.MustCompile(
	`\\(?:[Rr]ef|eqref|[Aa]utoref|[Cc]ref(?:range)?|labelcref|[Pp]ageref|[Nn]ameref|[Vv]ref|vpageref|autopageref)\*?` +
		`(?:\[[^\]]*\])*` +
		`\{([^}]+)\}`)

// reHyperref matches `\hyperref[name]` (square-bracket form).
var reHyperref = regexp.MustCompile(`\\hyperref\[([^\]]+)\]`)

// ignoredLabelPrefixes are spelled-out prefixes whose unused state is a
// LaTeX convention (sectioning + theorem-likes + proof-structure +
// textbook-style). Match is on the substring before the first ':'.
var ignoredLabelPrefixes = map[string]bool{
	"part":          true,
	"chapter":       true,
	"section":       true,
	"subsection":    true,
	"subsubsection": true,
	"paragraph":     true,
	"subparagraph":  true,
	"appendix":      true,
	"definition":    true,
	"theorem":       true,
	"corollary":     true,
	"lemma":         true,
	"proposition":   true,
	"example":       true,
	"remark":        true,
	"proof":         true,
	"claim":         true,
	"conjecture":    true,
	"axiom":         true,
	"fact":          true,
	"observation":   true,
	"note":          true,
	"assumption":    true,
	"hypothesis":    true,
	"property":      true,
	"exercise":      true,
	"problem":       true,
	"solution":      true,
	"case":          true,
}

type labelDef struct {
	path string
	line int
}

func checkUnusedLabels(files map[string][]string) []Diagnostic {
	defs := map[string][]labelDef{}
	refs := map[string]bool{}

	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		lines := files[path]
		mask := regionMask(lines)
		for i, line := range lines {
			rm := mask[i]
			for _, m := range reLabelCall.FindAllStringSubmatchIndex(line, -1) {
				if inVerbatim(rm, m[0]) {
					continue
				}
				name := strings.TrimSpace(line[m[2]:m[3]])
				if name == "" {
					continue
				}
				defs[name] = append(defs[name], labelDef{path: path, line: i + 1})
			}
			for _, m := range reRefCall.FindAllStringSubmatchIndex(line, -1) {
				if inVerbatim(rm, m[0]) {
					continue
				}
				for k := range strings.SplitSeq(line[m[2]:m[3]], ",") {
					k = strings.TrimSpace(k)
					if k != "" {
						refs[k] = true
					}
				}
			}
			for _, m := range reHyperref.FindAllStringSubmatchIndex(line, -1) {
				if inVerbatim(rm, m[0]) {
					continue
				}
				k := strings.TrimSpace(line[m[2]:m[3]])
				if k != "" {
					refs[k] = true
				}
			}
		}
	}

	names := make([]string, 0, len(defs))
	for n := range defs {
		names = append(names, n)
	}
	sort.Strings(names)

	var diags []Diagnostic
	for _, name := range names {
		if refs[name] || isIgnoredLabel(name) {
			continue
		}
		for _, d := range defs[name] {
			diags = append(diags, Diagnostic{
				File:    d.path,
				Line:    d.line,
				Message: fmt.Sprintf("unreferenced label: %s", name),
			})
		}
	}
	return diags
}

func inVerbatim(rm []regionKind, off int) bool {
	return off >= 0 && off < len(rm) && rm[off] == regVerbatim
}

func isIgnoredLabel(name string) bool {
	idx := strings.IndexByte(name, ':')
	if idx <= 0 {
		return false
	}
	return ignoredLabelPrefixes[name[:idx]]
}
