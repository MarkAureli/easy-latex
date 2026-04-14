package texscan

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reBibliography       = regexp.MustCompile(`\\bibliography\{([^}]+)\}`)
	reBibResource        = regexp.MustCompile(`\\addbibresource\{([^}]+)\}`)
	reInclude            = regexp.MustCompile(`\\(?:input|include)\{([^}]+)\}`)
	reFileContentsBegin  = regexp.MustCompile(`\\begin\{filecontents\*?\}(?:\[[^\]]*\])?\{([^}]+\.bib)\}`)
	reFileContentsEnd    = regexp.MustCompile(`\\end\{filecontents\*?\}`)
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
			line := StripComment(s.Text())

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

// FindTexFiles scans mainTex and recursively included .tex files and returns
// the path (relative to dir) of every visited file, including mainTex itself.
func FindTexFiles(mainTex, dir string) []string {
	seen := map[string]bool{}
	var result []string

	var walk func(name string)
	walk = func(name string) {
		path := filepath.Join(dir, name)
		if seen[path] {
			return
		}
		seen[path] = true
		result = append(result, path)

		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()

		s := bufio.NewScanner(f)
		for s.Scan() {
			line := StripComment(s.Text())
			if m := reInclude.FindStringSubmatch(line); m != nil {
				inc := strings.TrimSpace(m[1])
				if !strings.HasSuffix(inc, ".tex") {
					inc += ".tex"
				}
				walk(inc)
			}
		}
	}

	walk(mainTex)
	return result
}

// ResolveFileContents finds \begin{filecontents}{*.bib}...\end{filecontents} blocks
// in mainTex and all included .tex files, writes the embedded content to disk as the
// named .bib file, and removes the block from the tex file.
func ResolveFileContents(mainTex, dir string) error {
	for _, path := range FindTexFiles(mainTex, dir) {
		if err := resolveFileContentsInFile(path, dir); err != nil {
			return err
		}
	}
	return nil
}

func resolveFileContentsInFile(path, dir string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")

	var outLines []string
	changed := false
	i := 0
	for i < len(lines) {
		if m := reFileContentsBegin.FindStringSubmatch(StripComment(lines[i])); m != nil {
			bibName := m[1]
			var content []string
			i++
			for i < len(lines) {
				if reFileContentsEnd.MatchString(StripComment(lines[i])) {
					i++
					break
				}
				content = append(content, lines[i])
				i++
			}
			bibContent := strings.Join(content, "\n") + "\n"
			if err := os.WriteFile(filepath.Join(dir, bibName), []byte(bibContent), 0644); err != nil {
				return fmt.Errorf("cannot write %s: %w", bibName, err)
			}
			changed = true
			continue
		}
		outLines = append(outLines, lines[i])
		i++
	}

	if changed {
		if err := os.WriteFile(path, []byte(strings.Join(outLines, "\n")), 0644); err != nil {
			return fmt.Errorf("cannot write %s: %w", path, err)
		}
	}
	return nil
}

// RewriteBibReferences updates \bibliography and \addbibresource declarations in
// all tex files reachable from mainTex to reference newBibFiles instead of the
// old ones. The first occurrence in each file is replaced; subsequent occurrences
// of the same command type are dropped.
func RewriteBibReferences(mainTex, dir string, newBibFiles []string) error {
	for _, path := range FindTexFiles(mainTex, dir) {
		if err := rewriteBibRefsInFile(path, newBibFiles); err != nil {
			return err
		}
	}
	return nil
}

func rewriteBibRefsInFile(path string, newBibFiles []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")

	newNames := make([]string, len(newBibFiles))
	for i, f := range newBibFiles {
		newNames[i] = strings.TrimSuffix(f, ".bib")
	}
	newBibArg := strings.Join(newNames, ",")

	var outLines []string
	bibDone := false
	addBibDone := false
	changed := false

	for _, line := range lines {
		stripped := StripComment(line)

		if reBibliography.MatchString(stripped) {
			changed = true
			if !bibDone {
				outLines = append(outLines, reBibliography.ReplaceAllLiteralString(line, `\bibliography{`+newBibArg+`}`))
				bibDone = true
			}
			// subsequent occurrences dropped
			continue
		}

		if reBibResource.MatchString(stripped) {
			changed = true
			if !addBibDone {
				outLines = append(outLines, reBibResource.ReplaceAllLiteralString(line, `\addbibresource{`+newBibFiles[0]+`}`))
				for _, f := range newBibFiles[1:] {
					outLines = append(outLines, `\addbibresource{`+f+`}`)
				}
				addBibDone = true
			}
			continue
		}

		outLines = append(outLines, line)
	}

	if !changed {
		return nil
	}
	return os.WriteFile(path, []byte(strings.Join(outLines, "\n")), 0644)
}

// StripComment returns the portion of line before any unescaped % comment marker.
func StripComment(line string) string {
	idx := strings.Index(line, "%")
	if idx < 0 {
		return line
	}
	return line[:idx]
}
