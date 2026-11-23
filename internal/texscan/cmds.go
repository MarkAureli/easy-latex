package texscan

import (
	"maps"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	reCmdUse     = regexp.MustCompile(`\\([A-Za-z]+\*?)`)
	reNewCmd     = regexp.MustCompile(`\\(new|renew|provide)command\*?\s*(?:\{\s*\\([A-Za-z]+)\s*\}|\\([A-Za-z]+))\s*(?:\[(\d+)\])?\s*(\[)?`)
	reDefCmd     = regexp.MustCompile(`\\(?:def|gdef|edef|xdef)\s*\\([A-Za-z]+)((?:#[1-9])*)`)
	reDocCmd     = regexp.MustCompile(`\\(?:New|Renew|Provide|Declare)DocumentCommand\s*(?:\{\s*\\([A-Za-z]+)\s*\}|\\([A-Za-z]+))\s*\{`)
	reMathOp     = regexp.MustCompile(`\\DeclareMathOperator(\*?)\s*(?:\{\s*\\([A-Za-z]+)\s*\}|\\([A-Za-z]+))`)
	rePairedDelm = regexp.MustCompile(`\\DeclarePairedDelimiter(X?)\s*(?:\{\s*\\([A-Za-z]+)\s*\}|\\([A-Za-z]+))\s*(?:\[(\d+)\])?`)
)

// prepareTex returns the comment-stripped, verbatim-free text of a file.
func prepareTex(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	lines := strings.Split(string(data), "\n")
	for i, l := range lines {
		lines[i] = StripComment(l)
	}
	return reVerbatimEn.ReplaceAllString(strings.Join(lines, "\n"), ""), true
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// cmdDecls collects command declarations: name -> number of mandatory
// arguments, plus provide[name] = true for names declared only via
// \providecommand (which leaves an existing definition alone).
type cmdDecls struct {
	args    map[string]int
	provide map[string]bool
}

func newCmdDecls() cmdDecls {
	return cmdDecls{args: map[string]int{}, provide: map[string]bool{}}
}

func (d cmdDecls) set(name string, n int, provide bool) {
	d.args[name] = n
	if !provide {
		d.provide[name] = false
	} else if _, seen := d.provide[name]; !seen {
		d.provide[name] = true
	}
}

// provided returns the names declared only via \providecommand.
func (d cmdDecls) provided() map[string]bool {
	out := map[string]bool{}
	for name, p := range d.provide {
		if p {
			out[name] = true
		}
	}
	return out
}

// scanCmdDecls reads command declarations (\newcommand, \def,
// \NewDocumentCommand, \DeclareMathOperator, \DeclarePairedDelimiter) into d.
func scanCmdDecls(text string, d cmdDecls) {
	for _, m := range reNewCmd.FindAllStringSubmatch(text, -1) {
		n := 0
		if m[4] != "" {
			n, _ = strconv.Atoi(m[4])
			if m[5] != "" && n > 0 { // [n][default]: first arg optional
				n--
			}
		}
		d.set(firstNonEmpty(m[2], m[3]), n, m[1] == "provide")
	}
	for _, m := range reDefCmd.FindAllStringSubmatch(text, -1) {
		d.set(m[1], strings.Count(m[2], "#"), false)
	}
	for _, m := range reDocCmd.FindAllStringSubmatchIndex(text, -1) {
		spec, _, ok := balanced(text, m[1]-1, '{', '}')
		if !ok {
			continue
		}
		name := firstNonEmpty(text[max(m[2], 0):max(m[3], 0)], text[max(m[4], 0):max(m[5], 0)])
		d.set(name, DocumentSpecMandatory(spec), false)
	}
	for _, m := range reMathOp.FindAllStringSubmatch(text, -1) {
		d.set(firstNonEmpty(m[2], m[3]), 0, false)
	}
	for _, m := range rePairedDelm.FindAllStringSubmatch(text, -1) {
		n := 1
		if m[1] == "X" && m[4] != "" {
			n, _ = strconv.Atoi(m[4])
		}
		d.set(firstNonEmpty(m[2], m[3]), n, false)
	}
}

// ScanCmdDecls returns the mandatory-argument counts of commands declared in
// texFiles, and the names declared only via \providecommand.
func ScanCmdDecls(texFiles []string) (args map[string]int, provided map[string]bool) {
	d := newCmdDecls()
	for _, path := range texFiles {
		if text, ok := prepareTex(path); ok {
			scanCmdDecls(text, d)
		}
	}
	return d.args, d.provided()
}

// ScanCmds returns the number of mandatory arguments for every command used or
// declared in texFiles, resolved by ResolveCmd from the declarations, the
// compile probe's counts (probed, may be nil) and usage: the most frequent
// number of consecutive {...} groups directly following the command (optional
// [...] groups are skipped; ties go to the smaller count). The mode rather
// than the minimum is used because brace-less arguments (\mathcal H, \frac12)
// and bare mentions (\titleformat{\section}) are common and would otherwise
// pin the count to 0.
func ScanCmds(texFiles []string, probed map[string]int) map[string]int {
	used := map[string]map[int]int{}
	d := newCmdDecls()
	for _, path := range texFiles {
		text, ok := prepareTex(path)
		if !ok {
			continue
		}
		scanCmdDecls(text, d)
		for _, m := range reCmdUse.FindAllStringSubmatchIndex(text, -1) {
			name := text[m[2]:m[3]]
			n := countArgGroups(text, m[1])
			if used[name] == nil {
				used[name] = map[int]int{}
			}
			used[name][n]++
		}
	}
	usage := map[string]int{}
	for name, hist := range used {
		best, bestN := -1, 0
		for n, c := range hist {
			if c > bestN || (c == bestN && n < best) {
				best, bestN = n, c
			}
		}
		usage[name] = best
	}
	for name := range d.args {
		if _, ok := usage[name]; !ok {
			usage[name] = d.args[name]
		}
	}
	provided := d.provided()
	out := map[string]int{}
	for name := range usage {
		out[name] = ResolveCmd(name, d.args, provided, probed, usage)
	}
	return out
}

// ResolveCmd returns the mandatory-argument count of name from its layers, in
// precedence order: project declaration (except \providecommand when the probe
// resolved it), compile probe, built-in table (BuiltinCmdArgs), usage. A
// command no layer knows yields 0.
func ResolveCmd(name string, declared map[string]int, provided map[string]bool, probed, usage map[string]int) int {
	p, probeOK := probed[name]
	if n, ok := declared[name]; ok && !(provided[name] && probeOK) {
		return n
	}
	if probeOK {
		return p
	}
	if n, ok := BuiltinCmdArgs[name]; ok {
		return n
	}
	return usage[name]
}

// storedCmdArgs returns the non-zero entries of the command table at path
// (.el/cmds.json). Zeros are skipped like unresolved probe results: a table
// written before the built-in layer existed may hold usage counts such as
// frac: 0 from \frac12, and a 0 never leads to an action anyway.
func storedCmdArgs(path string) map[string]int {
	out := map[string]int{}
	for name, n := range LoadTable[int](path) {
		if n > 0 {
			out[name] = n
		}
	}
	return out
}

// StoredCmdArgs returns BuiltinCmdArgs overlaid by the command table at path,
// except that a stored 0 never overrides a built-in count. A missing file
// yields the built-in table alone. Stored zeros are kept otherwise so that
// argument-free commands (\alpha) are known. Cheap; used by el lsp, whose
// on-save refresh keeps declarations in the table.
func StoredCmdArgs(path string) map[string]int {
	out := maps.Clone(BuiltinCmdArgs)
	for name, n := range LoadTable[int](path) {
		if _, builtin := out[name]; n > 0 || !builtin {
			out[name] = n
		}
	}
	return out
}

// ProjectCmdArgs returns the full command table for checks run by el check /
// el compile: StoredCmdArgs overlaid by ScanCmds over texFiles, with the stored
// entries standing in for the probe layer. Fresh declarations thus win and
// commands new since the last compile get usage counts.
func ProjectCmdArgs(path string, texFiles []string) map[string]int {
	out := StoredCmdArgs(path)
	maps.Copy(out, ScanCmds(texFiles, storedCmdArgs(path)))
	return out
}
