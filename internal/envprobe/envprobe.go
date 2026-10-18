// Package envprobe instruments a real compile to learn what kind of
// environments the document uses. A generated .sty hooks the begin and end of
// each candidate environment and logs TeX's own state (\@captype, \@listdepth)
// to <jobname>.elprobe; Parse turns that log into per-environment verdicts.
package envprobe

import (
	"bufio"
	"os"
	"sort"
	"strconv"
	"strings"
)

// PkgName is the name (without .sty) of the generated probe package.
const PkgName = "el-envprobe"

// Ext is the extension of the side file the probe writes next to the .aux.
const Ext = ".elprobe"

// builtin lists environments probed even before the document uses them.
var builtin = []string{"figure", "figure*", "table", "table*", "itemize", "enumerate", "description"}

// Result is the verdict for one environment.
type Result struct {
	Float bool // entering it sets \@captype (it goes through \@float)
	List  bool // entering it raises \@listdepth (it goes through \list)
}

// Sty returns the probe package source for the given environment names (the
// built-in set is added). Hooks need the LaTeX kernel's \AddToHook (2020-10+);
// on older kernels the package does nothing.
func Sty(names []string) []byte {
	set := map[string]bool{}
	for _, n := range append(append([]string{}, builtin...), names...) {
		if n != "document" && n != "" {
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
\newwrite\elprobe@file
\immediate\openout\elprobe@file=\jobname` + Ext + `\relax
\def\elprobe@state{\ifdefined\@captype\@captype\else-\fi,\number\@listdepth}
\newcommand\elprobe[1]{%
  \AddToHook{env/#1/begin}{\edef\elprobe@outer{\elprobe@state}}%
  \AddToHook{env/#1/end}{\immediate\write\elprobe@file{#1|\elprobe@outer|\elprobe@state}}}
`)
	for _, n := range sorted {
		b.WriteString(`\elprobe{` + n + "}\n")
	}
	b.WriteString("\\fi\n")
	return []byte(b.String())
}

// Parse reads a probe log. Entering an environment is a float when \@captype
// becomes defined (or changes) and a list when \@listdepth grows, so wrappers
// nested in a float or list are not misclassified. Verdicts are OR-ed over all
// occurrences. A missing file yields an empty result.
func Parse(path string) map[string]Result {
	out := map[string]Result{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Split(strings.TrimSpace(sc.Text()), "|")
		if len(parts) != 3 {
			continue
		}
		oc, od, ok1 := splitState(parts[1])
		ic, id, ok2 := splitState(parts[2])
		if !ok1 || !ok2 {
			continue
		}
		r := out[parts[0]]
		r.Float = r.Float || (ic != "-" && ic != oc)
		r.List = r.List || id > od
		out[parts[0]] = r
	}
	return out
}

func splitState(s string) (captype string, depth int, ok bool) {
	i := strings.LastIndexByte(s, ',')
	if i < 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(s[i+1:])
	return s[:i], n, err == nil
}
