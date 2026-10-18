package texscan

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	reEnvBegin   = regexp.MustCompile(`\\begin\{([A-Za-z]+\*?)\}`)
	reNewEnv     = regexp.MustCompile(`\\(?:new|renew|provide)environment\*?\s*\{([A-Za-z]+\*?)\}\s*(?:\[(\d+)\])?\s*(\[)?`)
	reNewDocEnv  = regexp.MustCompile(`\\(?:New|Renew|Provide|Declare)DocumentEnvironment\s*\{([A-Za-z]+\*?)\}\s*\{`)
	reVerbatimEn = regexp.MustCompile(`(?s)\\begin\{(verbatim|Verbatim|lstlisting|minted|comment|alltt)\*?\}.*?\\end\{(?:verbatim|Verbatim|lstlisting|minted|comment|alltt)\*?\}`)
)

// EnvSignatures returns the number of mandatory arguments of environments
// declared or used in texFiles. Declared environments (\newenvironment,
// \NewDocumentEnvironment) use their declared count. Other environments take
// the minimum number of consecutive {...} groups directly following
// \begin{env} over all occurrences (optional [...] groups are skipped):
// a body starting with a brace group can only inflate single occurrences.
func EnvSignatures(texFiles []string) map[string]int {
	used := map[string]int{}
	declared := map[string]int{}
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
	return used
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
