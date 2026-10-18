package texscan

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	reEnvBegin   = regexp.MustCompile(`\\begin\{([A-Za-z]+\*?)\}`)
	reNewEnv     = regexp.MustCompile(`\\(?:new|renew|provide)environment\*?\s*\{([A-Za-z]+\*?)\}\s*(?:\[(\d+)\])?\s*(\[)?`)
	reNewDocEnv  = regexp.MustCompile(`\\(?:New|Renew|Provide|Declare)DocumentEnvironment\s*\{([A-Za-z]+\*?)\}\s*\{`)
	reNewTheorem = regexp.MustCompile(`\\(?:newtheorem|newmdtheoremenv|declaretheorem|spnewtheorem|newtcbtheorem)\*?\s*`)
	reVerbatimEn = regexp.MustCompile(`(?s)\\begin\{(verbatim|Verbatim|lstlisting|minted|comment|alltt)\*?\}.*?\\end\{(?:verbatim|Verbatim|lstlisting|minted|comment|alltt)\*?\}`)
)

// EnvInfo describes an environment found in the project sources.
type EnvInfo struct {
	Args    int  `json:"args"`    // mandatory arguments
	Theorem bool `json:"theorem"` // declared via \newtheorem & co.
}

// FindStyFiles returns the .sty and .cls files below dir, skipping hidden
// directories (including .el).
func FindStyFiles(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext == ".sty" || ext == ".cls" {
			out = append(out, path)
		}
		return nil
	})
	return out
}

// ScanEnvs collects environments declared or used in texFiles.
//
// Mandatory-argument counts: declared environments (\newenvironment,
// \NewDocumentEnvironment) use their declared count; others take the minimum
// number of consecutive {...} groups directly following \begin{env} over all
// occurrences (optional [...] groups are skipped), since a body starting with
// a brace group can only inflate single occurrences.
//
// Theorem: set for names declared by \newtheorem, \newmdtheoremenv,
// \declaretheorem, \spnewtheorem or \newtcbtheorem.
func ScanEnvs(texFiles []string) map[string]EnvInfo {
	used := map[string]int{}
	declared := map[string]int{}
	theorems := map[string]bool{}
	for _, path := range texFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for i, l := range lines {
			lines[i] = StripComment(l)
		}
		text := reVerbatimEn.ReplaceAllString(strings.Join(lines, "\n"), "")

		for _, m := range reNewEnv.FindAllStringSubmatchIndex(text, -1) {
			n := 0
			if m[4] >= 0 {
				n, _ = strconv.Atoi(text[m[4]:m[5]])
				if m[6] >= 0 && n > 0 { // [n][default]: first arg optional
					n--
				}
			}
			declared[text[m[2]:m[3]]] = n
		}
		for _, m := range reNewDocEnv.FindAllStringSubmatchIndex(text, -1) {
			spec, _, ok := balanced(text, m[1]-1, '{', '}')
			if !ok {
				continue
			}
			declared[text[m[2]:m[3]]] = documentSpecMandatory(spec)
		}
		for _, m := range reNewTheorem.FindAllStringIndex(text, -1) {
			pos := m[1]
			if _, end, ok := balanced(text, pos, '[', ']'); ok {
				pos = end
				for pos < len(text) && strings.IndexByte(" \t\r\n", text[pos]) >= 0 {
					pos++
				}
			}
			names, _, ok := balanced(text, pos, '{', '}')
			if !ok {
				continue
			}
			for _, name := range strings.FieldsFunc(names, func(r rune) bool { return r == ',' || r == ' ' }) {
				theorems[name] = true
			}
		}
		for _, m := range reEnvBegin.FindAllStringSubmatchIndex(text, -1) {
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
	out := map[string]EnvInfo{}
	for name, n := range used {
		out[name] = EnvInfo{Args: n, Theorem: theorems[name]}
	}
	for name := range theorems {
		if _, ok := out[name]; !ok {
			out[name] = EnvInfo{Theorem: true}
		}
	}
	return out
}

// balanced returns the content of the group opening at text[i] and the index
// just past its closing delimiter.
func balanced(text string, i int, open, closeCh byte) (string, int, bool) {
	if i >= len(text) || text[i] != open {
		return "", i, false
	}
	depth := 0
	for j := i; j < len(text); j++ {
		switch {
		case text[j] == '\\':
			j++
		case text[j] == open:
			depth++
		case text[j] == closeCh:
			depth--
			if depth == 0 {
				return text[i+1 : j], j + 1, true
			}
		}
	}
	return "", i, false
}

// countArgGroups counts consecutive {...} groups after text[pos:], skipping
// whitespace (but not paragraph breaks) and [...] groups.
func countArgGroups(text string, pos int) int {
	n := 0
	for {
		nl := 0
		for pos < len(text) && strings.IndexByte(" \t\r\n", text[pos]) >= 0 {
			if text[pos] == '\n' {
				nl++
			}
			pos++
		}
		if nl > 1 || pos >= len(text) {
			return n
		}
		switch text[pos] {
		case '[':
			_, end, ok := balanced(text, pos, '[', ']')
			if !ok {
				return n
			}
			pos = end
		case '{':
			_, end, ok := balanced(text, pos, '{', '}')
			if !ok {
				return n
			}
			n++
			pos = end
		default:
			return n
		}
	}
}

// documentSpecMandatory counts mandatory arguments (m, v, r, R) in an xparse
// argument specification, ignoring brace-delimited defaults.
func documentSpecMandatory(spec string) int {
	n := 0
	for i := 0; i < len(spec); i++ {
		switch spec[i] {
		case '{':
			if _, end, ok := balanced(spec, i, '{', '}'); ok {
				i = end - 1
			}
		case 'm', 'v', 'r', 'R':
			n++
		}
	}
	return n
}
