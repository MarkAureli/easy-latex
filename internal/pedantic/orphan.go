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

//go:embed el-orphan.sty
var OrphanSty []byte

func init() {
	Register(Check{
		Name:        "no-orphan-line",
		Phase:       PhasePostCompile,
		PostCompile: checkOrphanLine,
		StyName:     "el-orphan.sty",
		Sty:         OrphanSty,
	})
}

type orphanStart struct {
	YPos int
	Line int
	Page int
}

type orphanEnd struct {
	LineCount int
	Page      int
}

// parseOrphan reads a .orphan file and returns per-id start/end records.
// Page assignment uses the most recent P record seen before each S or E.
func parseOrphan(path string) (starts map[int]orphanStart, ends map[int]orphanEnd, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	starts = make(map[int]orphanStart)
	ends = make(map[int]orphanEnd)
	currentPage := 0

	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "P":
			if p, perr := strconv.Atoi(fields[1]); perr == nil {
				currentPage = p
			}
		case "S":
			if len(fields) < 4 {
				continue
			}
			id, e1 := strconv.Atoi(fields[1])
			y, e2 := strconv.Atoi(fields[2])
			lineNo, e3 := strconv.Atoi(fields[3])
			if e1 != nil || e2 != nil || e3 != nil {
				continue
			}
			starts[id] = orphanStart{YPos: y, Line: lineNo, Page: currentPage}
		case "E":
			if len(fields) < 3 {
				continue
			}
			id, e1 := strconv.Atoi(fields[1])
			lc, e2 := strconv.Atoi(fields[2])
			if e1 != nil || e2 != nil {
				continue
			}
			ends[id] = orphanEnd{LineCount: lc, Page: currentPage}
		}
	}
	return starts, ends, sc.Err()
}

func checkOrphanLine(auxDir string) []Diagnostic {
	stem := findOrphanStem(auxDir)
	if stem == "" {
		return nil
	}
	starts, ends, err := parseOrphan(filepath.Join(auxDir, stem+".orphan"))
	if err != nil {
		return nil
	}
	mainTex := stem + ".tex"

	var diags []Diagnostic
	for id, s := range starts {
		e, ok := ends[id]
		if !ok {
			continue
		}
		if s.Page == e.Page {
			continue
		}
		// Only flag the definite case: a 2-line paragraph split across pages
		// is necessarily an orphan (line 1 alone on the start page, line 2
		// on the next).  Longer split paragraphs may be orphans, widows, or
		// mid-paragraph breaks; we cannot disambiguate without per-line data.
		if e.LineCount != 2 {
			continue
		}
		diags = append(diags, Diagnostic{
			File:    mainTex,
			Line:    s.Line,
			Message: fmt.Sprintf("orphan: 2-line paragraph split across pages %d and %d", s.Page, e.Page),
		})
	}
	return diags
}

// findOrphanStem returns the TeX stem from auxDir by locating a .orphan file.
func findOrphanStem(auxDir string) string {
	entries, err := os.ReadDir(auxDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if stem, ok := strings.CutSuffix(e.Name(), ".orphan"); ok {
			return stem
		}
	}
	return ""
}
