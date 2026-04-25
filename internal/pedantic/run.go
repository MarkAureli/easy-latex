package pedantic

import (
	"bufio"
	"os"
	"sort"
	"strings"

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

// HasFixableChecks returns true if any enabled check provides a Fix.
func HasFixableChecks(checkNames []string) bool {
	for _, name := range checkNames {
		if c, ok := Get(name); ok && c.Fix != nil {
			return true
		}
	}
	return false
}

// RunSourceFixes applies all enabled fixable checks to texFiles in-place.
// Returns sorted list of paths that were actually modified.
func RunSourceFixes(checkNames, texFiles []string) ([]string, error) {
	var fixes []Check
	for _, name := range checkNames {
		if c, ok := Get(name); ok && c.Fix != nil {
			fixes = append(fixes, c)
		}
	}
	if len(fixes) == 0 {
		return nil, nil
	}
	var modified []string
	for _, path := range texFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return modified, err
		}
		lines := strings.Split(string(data), "\n")
		changed := false
		for _, c := range fixes {
			if newLines, did := c.Fix(path, lines); did {
				lines = newLines
				changed = true
			}
		}
		if changed {
			if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
				return modified, err
			}
			modified = append(modified, path)
		}
	}
	sort.Strings(modified)
	return modified, nil
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
