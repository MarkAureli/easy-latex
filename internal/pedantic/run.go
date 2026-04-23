package pedantic

import (
	"bufio"
	"os"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// RunSourceChecks runs all enabled source-level checks on the given tex files.
func RunSourceChecks(checkNames, texFiles []string) []Diagnostic {
	var checks []Check
	for _, name := range checkNames {
		if c, ok := Get(name); ok && c.Phase == PhaseSource {
			checks = append(checks, c)
		}
	}
	if len(checks) == 0 {
		return nil
	}
	var all []Diagnostic
	for _, path := range texFiles {
		lines := readAndStripComments(path)
		for _, c := range checks {
			all = append(all, c.Source(path, lines)...)
		}
	}
	return all
}

// RunPostCompileChecks runs all enabled post-compile checks using synctex data.
func RunPostCompileChecks(checkNames []string, synctexPath string, texFiles []string) ([]Diagnostic, error) {
	var checks []Check
	for _, name := range checkNames {
		if c, ok := Get(name); ok && c.Phase == PhasePostCompile {
			checks = append(checks, c)
		}
	}
	if len(checks) == 0 {
		return nil, nil
	}
	stx, err := ParseSynctex(synctexPath)
	if err != nil {
		return nil, err
	}
	sources := make(map[string][]string, len(texFiles))
	for _, path := range texFiles {
		sources[path] = readAndStripComments(path)
	}
	var all []Diagnostic
	for _, c := range checks {
		all = append(all, c.Post(stx, sources)...)
	}
	return all, nil
}

// readAndStripComments reads a file and returns lines with comments stripped.
func readAndStripComments(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, texscan.StripComment(sc.Text()))
	}
	return lines
}
