package texscan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reBibliography = regexp.MustCompile(`\\bibliography\{([^}]+)\}`)
	reBibResource  = regexp.MustCompile(`\\addbibresource\{([^}]+)\}`)
	reInclude      = regexp.MustCompile(`\\(?:input|include)\{([^}]+)\}`)
)

// FindBibFiles scans mainTex (and recursively included .tex files) for
// bibliography declarations and returns the referenced .bib file names
// relative to dir.
func FindBibFiles(mainTex, dir string) []string {
	seen := map[string]bool{}
	var result []string

	var scan func(name string)
	scan = func(name string) {
		path := filepath.Join(dir, name)
		if seen[path] {
			return
		}
		seen[path] = true

		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()

		s := bufio.NewScanner(f)
		for s.Scan() {
			line := stripComment(s.Text())

			if m := reBibliography.FindStringSubmatch(line); m != nil {
				for _, raw := range strings.Split(m[1], ",") {
					addBibFile(&result, strings.TrimSpace(raw))
				}
			}
			if m := reBibResource.FindStringSubmatch(line); m != nil {
				addBibFile(&result, strings.TrimSpace(m[1]))
			}
			if m := reInclude.FindStringSubmatch(line); m != nil {
				inc := strings.TrimSpace(m[1])
				if !strings.HasSuffix(inc, ".tex") {
					inc += ".tex"
				}
				scan(inc)
			}
		}
	}

	scan(mainTex)
	return result
}

func addBibFile(result *[]string, name string) {
	if !strings.HasSuffix(name, ".bib") {
		name += ".bib"
	}
	for _, v := range *result {
		if v == name {
			return
		}
	}
	*result = append(*result, name)
}

func stripComment(line string) string {
	idx := strings.Index(line, "%")
	if idx < 0 {
		return line
	}
	return line[:idx]
}
