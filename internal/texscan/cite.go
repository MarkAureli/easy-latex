package texscan

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strings"
)

// ReCiteCall matches a LaTeX citation command including optional args and the key group.
// Covers: \cite, \citep, \citet, \citeauthor, \parencite, \textcite,
// \autocite, \fullcite, \citealt, \citealp, \Cite, \Citep, \Citet,
// and starred variants. Optional arguments ([...]) are skipped.
// ReCiteCall matches a LaTeX citation command including optional args and the key group.
var ReCiteCall = regexp.MustCompile(
	`(?:\\[Cc]ite(?:p|t|author|alt|alp)?|\\(?:parencite|textcite|autocite|fullcite))\*?` +
		`(?:\[[^\]]*\])*` + // optional arguments
		`\{([^}]+)\}`)

// FindCiteKeys returns sorted, deduplicated citation keys from all .tex files
// reachable from mainTex. Comments are stripped before matching.
func FindCiteKeys(mainTex, dir string) []string {
	texFiles := FindTexFiles(mainTex, dir)
	seen := map[string]bool{}
	for _, path := range texFiles {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := StripComment(scanner.Text())
			for _, m := range ReCiteCall.FindAllStringSubmatch(line, -1) {
				for _, key := range strings.Split(m[1], ",") {
					key = strings.TrimSpace(key)
					if key != "" {
						seen[key] = true
					}
				}
			}
		}
		f.Close()
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
