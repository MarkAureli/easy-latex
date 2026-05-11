package pedantic

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// reTitleCall locates the source line of \title{...} in a tex file. Used to
// recover the line for kind="title" records (which carry line=0 because the
// wrap happens at \AtBeginDocument, after the preamble \title call).
var reTitleCall = regexp.MustCompile(`\\title\b`)

//go:embed el-sectionpos.sty
var SectionPosSty []byte

func init() {
	Register(Check{
		Name:        "no-section-linebreak",
		Phase:       PhasePostCompile,
		PostCompile: checkSectionLinebreak,
		StyName:     "el-sectionpos.sty",
		Sty:         SectionPosSty,
	})
}

// sectionPosM is an M record: call-site line + kind.
type sectionPosM struct {
	Line int
	Kind string
}

// sectionPosY is an S or E record: y-position.
type sectionPosY struct {
	YPos int
}

// parseSectionPos reads a .sectionpos file. Returns:
//
//	marks   id → M record  (call-site source line + kind)
//	starts  id → S record  (y at title typesetting start)
//	ends    id → E record  (y at title typesetting end)
func parseSectionPos(path string) (marks map[int]sectionPosM, starts, ends map[int]sectionPosY, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()

	marks = make(map[int]sectionPosM)
	starts = make(map[int]sectionPosY)
	ends = make(map[int]sectionPosY)

	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		kind := fields[0]
		id, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		switch kind {
		case "M":
			srcLine, err := strconv.Atoi(fields[2])
			if err != nil {
				continue
			}
			marks[id] = sectionPosM{Line: srcLine, Kind: fields[3]}
		case "S":
			y, err := strconv.Atoi(fields[2])
			if err != nil {
				continue
			}
			starts[id] = sectionPosY{YPos: y}
		case "E":
			y, err := strconv.Atoi(fields[2])
			if err != nil {
				continue
			}
			ends[id] = sectionPosY{YPos: y}
		}
	}
	return marks, starts, ends, sc.Err()
}

func checkSectionLinebreak(auxDir string) []Diagnostic {
	stem := findSectionStem(auxDir)
	if stem == "" {
		return nil
	}
	sectionposPath := filepath.Join(auxDir, stem+".sectionpos")

	marks, starts, ends, err := parseSectionPos(sectionposPath)
	if err != nil {
		return nil
	}

	mainTex := stem + ".tex"
	titleLine := 0 // resolved lazily

	var diags []Diagnostic
	for id, m := range marks {
		s, sok := starts[id]
		e, eok := ends[id]
		if !sok || !eok {
			continue
		}
		if s.YPos == e.YPos {
			continue
		}
		line := m.Line
		if m.Kind == "title" && line == 0 {
			if titleLine == 0 {
				titleLine = findTitleLine(mainTex)
			}
			line = titleLine
		}
		diags = append(diags, Diagnostic{
			File:    mainTex,
			Line:    line,
			Message: fmt.Sprintf("%s spans multiple PDF lines", m.Kind),
		})
	}
	return diags
}

// findTitleLine returns the 1-based line of the first \title{ occurrence in
// the file, or 0 if not found / file unreadable.
func findTitleLine(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		if reTitleCall.MatchString(sc.Text()) {
			return n
		}
	}
	return 0
}

// findSectionStem returns the TeX stem from auxDir by locating a .sectionpos file.
func findSectionStem(auxDir string) string {
	entries, err := os.ReadDir(auxDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if stem, ok := strings.CutSuffix(e.Name(), ".sectionpos"); ok {
			return stem
		}
	}
	return ""
}
