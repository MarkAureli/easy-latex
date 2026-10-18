package texscan

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	reCmdUse     = regexp.MustCompile(`\\([A-Za-z]+\*?)`)
	reNewCmd     = regexp.MustCompile(`\\(?:new|renew|provide)command\*?\s*(?:\{\s*\\([A-Za-z]+)\s*\}|\\([A-Za-z]+))\s*(?:\[(\d+)\])?\s*(\[)?`)
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

// scanCmdDecls reads command declarations (\newcommand, \def,
// \NewDocumentCommand, \DeclareMathOperator, \DeclarePairedDelimiter) into
// declared: name -> number of mandatory arguments.
func scanCmdDecls(text string, declared map[string]int) {
	for _, m := range reNewCmd.FindAllStringSubmatch(text, -1) {
		n := 0
		if m[3] != "" {
			n, _ = strconv.Atoi(m[3])
			if m[4] != "" && n > 0 { // [n][default]: first arg optional
				n--
			}
		}
		declared[firstNonEmpty(m[1], m[2])] = n
	}
	for _, m := range reDefCmd.FindAllStringSubmatch(text, -1) {
		declared[m[1]] = strings.Count(m[2], "#")
	}
	for _, m := range reDocCmd.FindAllStringSubmatchIndex(text, -1) {
		spec, _, ok := balanced(text, m[1]-1, '{', '}')
		if !ok {
			continue
		}
		name := firstNonEmpty(text[max(m[2], 0):max(m[3], 0)], text[max(m[4], 0):max(m[5], 0)])
		declared[name] = documentSpecMandatory(spec)
	}
	for _, m := range reMathOp.FindAllStringSubmatch(text, -1) {
		declared[firstNonEmpty(m[2], m[3])] = 0
	}
	for _, m := range rePairedDelm.FindAllStringSubmatch(text, -1) {
		n := 1
		if m[1] == "X" && m[4] != "" {
			n, _ = strconv.Atoi(m[4])
		}
		declared[firstNonEmpty(m[2], m[3])] = n
	}
}

// ScanCmdDecls returns the mandatory-argument counts of commands declared in
// texFiles.
func ScanCmdDecls(texFiles []string) map[string]int {
	declared := map[string]int{}
	for _, path := range texFiles {
		if text, ok := prepareTex(path); ok {
			scanCmdDecls(text, declared)
		}
	}
	return declared
}

// ScanCmds returns the number of mandatory arguments per command: the declared
// count where a declaration exists, otherwise the minimum number of
// consecutive {...} groups directly following the command over all
// occurrences (optional [...] groups are skipped; a use followed by a body
// that starts with a brace group can only inflate single occurrences).
func ScanCmds(texFiles []string) map[string]int {
	used := map[string]int{}
	declared := map[string]int{}
	for _, path := range texFiles {
		text, ok := prepareTex(path)
		if !ok {
			continue
		}
		scanCmdDecls(text, declared)
		for _, m := range reCmdUse.FindAllStringSubmatchIndex(text, -1) {
			name := text[m[2]:m[3]]
			n := countArgGroups(text, m[1])
			if cur, ok := used[name]; !ok || n < cur {
				used[name] = n
			}
		}
	}
	for name, n := range declared {
		used[name] = n
	}
	return used
}
