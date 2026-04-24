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

// RunPostCompileChecks runs all enabled post-compile checks.
func RunPostCompileChecks(checkNames []string, auxDir string) []Diagnostic {
	var checks []Check
	for _, name := range checkNames {
		if c, ok := Get(name); ok && c.Phase == PhasePostCompile {
			checks = append(checks, c)
		}
	}
	if len(checks) == 0 {
		return nil
	}
	var all []Diagnostic
	for _, c := range checks {
		all = append(all, c.PostCompile(auxDir)...)
	}
	return all
}

// HasPostCompileChecks returns true if any enabled check is post-compile phase.
func HasPostCompileChecks(checkNames []string) bool {
	for _, name := range checkNames {
		if c, ok := Get(name); ok && c.Phase == PhasePostCompile {
			return true
		}
	}
	return false
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
