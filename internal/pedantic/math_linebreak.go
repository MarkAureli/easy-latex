package pedantic

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed el-mathpos.sty
var MathPosSty []byte

func init() {
	Register(Check{
		Name:        "no-math-linebreak",
		Phase:       PhasePostCompile,
		PostCompile: checkMathLinebreak,
	})
}

// mathPosEntry is one S or E record from the .mathpos file.
type mathPosEntry struct {
	ID   int
	YPos int
	Line int
}

// parseMathPos reads a .mathpos file and returns start/end entries keyed by ID.
func parseMathPos(path string) (starts, ends map[int]mathPosEntry, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	starts = make(map[int]mathPosEntry)
	ends = make(map[int]mathPosEntry)

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		e, kind, err := parseMathPosLine(line)
		if err != nil {
			continue // skip malformed lines
		}
		switch kind {
		case "S":
			starts[e.ID] = e
		case "E":
			ends[e.ID] = e
		}
	}
	return starts, ends, sc.Err()
}

// parseMathPosLine parses "S 1 43234099 42" or "E 1 43234099 42".
func parseMathPosLine(line string) (mathPosEntry, string, error) {
	fields := strings.Fields(line)
	if len(fields) != 4 {
		return mathPosEntry{}, "", fmt.Errorf("expected 4 fields, got %d", len(fields))
	}
	kind := fields[0]
	if kind != "S" && kind != "E" {
		return mathPosEntry{}, "", fmt.Errorf("unknown kind %q", kind)
	}
	id, err := strconv.Atoi(fields[1])
	if err != nil {
		return mathPosEntry{}, "", err
	}
	ypos, err := strconv.Atoi(fields[2])
	if err != nil {
		return mathPosEntry{}, "", err
	}
	srcLine, err := strconv.Atoi(fields[3])
	if err != nil {
		return mathPosEntry{}, "", err
	}
	return mathPosEntry{ID: id, YPos: ypos, Line: srcLine}, kind, nil
}

func checkMathLinebreak(auxDir string) []Diagnostic {
	stem := findStem(auxDir)
	if stem == "" {
		return nil
	}
	mathposPath := filepath.Join(auxDir, stem+".mathpos")

	starts, ends, err := parseMathPos(mathposPath)
	if err != nil {
		return nil // no .mathpos file — sty not injected or no math
	}

	// Read the main tex file to validate reported source lines.
	// Only flag violations where the source line actually contains
	// inline math delimiters ($ or \().  This filters out false
	// positives from \maketitle, bibliography entries, etc.
	mainTex := stem + ".tex"
	texLines := readLines(mainTex)

	// Classify entries: per source line, collect the max ID among
	// non-violating (same y) pairs.  Bibliography/macro math is
	// typeset after the document body, so its IDs are higher than
	// body math on the same source line.  A violation whose ID
	// exceeds the max non-violating ID for its line is a false
	// positive (e.g. $S_n$ in a bib title whose stale \inputlineno
	// happens to point at a body line containing $).
	maxCleanID := map[int]int{} // source line → max non-violating ID
	for id, s := range starts {
		e, ok := ends[id]
		if !ok {
			continue
		}
		if s.YPos == e.YPos {
			if id > maxCleanID[s.Line] {
				maxCleanID[s.Line] = id
			}
		}
	}

	var diags []Diagnostic
	for id, s := range starts {
		e, ok := ends[id]
		if !ok {
			continue
		}
		if s.YPos == e.YPos {
			continue
		}
		if !lineHasInlineMath(texLines, s.Line) {
			continue
		}
		// Skip if this violation's ID exceeds the highest
		// non-violating ID on the same source line — it comes
		// from bibliography or macro expansion, not the body.
		if cap, ok := maxCleanID[s.Line]; ok && id > cap {
			continue
		}
		diags = append(diags, Diagnostic{
			File:    mainTex,
			Line:    s.Line,
			Message: "inline math spans multiple PDF lines",
		})
	}
	return diags
}

// readLines reads all lines from a file, returning nil on error.
func readLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

// lineHasInlineMath returns true if the given 1-based source line
// contains an inline math delimiter ($ or \().
func lineHasInlineMath(lines []string, lineNo int) bool {
	if lineNo < 1 || lineNo > len(lines) {
		return false
	}
	line := lines[lineNo-1]
	return strings.Contains(line, "$") || strings.Contains(line, `\(`)
}

// findStem returns the TeX stem name from the aux directory by looking for a .mathpos file.
func findStem(auxDir string) string {
	entries, err := os.ReadDir(auxDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if stem, ok := strings.CutSuffix(e.Name(), ".mathpos"); ok {
			return stem
		}
	}
	return ""
}
