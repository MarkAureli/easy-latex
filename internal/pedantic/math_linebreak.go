package pedantic

import (
	"path/filepath"
	"regexp"
	"strings"
)

func init() {
	Register(Check{
		Name:  "no-math-linebreak",
		Phase: PhasePostCompile,
		Post:  checkMathLinebreak,
	})
}

// reInlineMath matches $...$ (not $$) or \(...\) on a source line.
var reInlineMath = regexp.MustCompile(`(?:\$[^$].*?\$|\\\(.*?\\\))`)

// hasInlineMath reports whether a source line contains inline math.
func hasInlineMath(line string) bool {
	if !strings.ContainsAny(line, "$\\") {
		return false
	}
	if strings.Contains(line, "$$") {
		return false
	}
	return reInlineMath.MatchString(line)
}

func checkMathLinebreak(stx *SynctexData, sources map[string][]string) []Diagnostic {
	// Build absolute path → source lines lookup
	absSource := make(map[string][]string, len(sources))
	for p, lines := range sources {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		absSource[abs] = lines
	}

	// Build synctex input tag → absolute path + source lines
	type fileInfo struct {
		absPath string
		lines   []string
	}
	tagInfo := make(map[int]*fileInfo)
	for tag, stxPath := range stx.Inputs {
		abs, err := filepath.Abs(stxPath)
		if err != nil {
			abs = stxPath
		}
		if lines, ok := absSource[abs]; ok {
			tagInfo[tag] = &fileInfo{absPath: abs, lines: lines}
		}
	}

	type fileLine struct {
		file string
		line int
	}
	reported := map[fileLine]bool{}
	var diags []Diagnostic

	for _, pair := range stx.MathPairs() {
		if pair.Open.V == pair.Close.V {
			continue
		}
		info := tagInfo[pair.Open.Input]
		if info == nil {
			continue
		}
		srcLine := pair.Open.Line
		if srcLine < 1 || srcLine > len(info.lines) {
			continue
		}
		if !hasInlineMath(info.lines[srcLine-1]) {
			continue
		}

		fl := fileLine{info.absPath, srcLine}
		if reported[fl] {
			continue
		}
		reported[fl] = true

		// Use the original relative path from sources for display
		displayPath := info.absPath
		for p := range sources {
			abs, _ := filepath.Abs(p)
			if abs == info.absPath {
				displayPath = p
				break
			}
		}

		diags = append(diags, Diagnostic{
			File:    displayPath,
			Line:    srcLine,
			Message: "inline math spans multiple PDF lines",
		})
	}
	return diags
}
