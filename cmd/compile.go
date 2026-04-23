package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/bib"
	"github.com/MarkAureli/easy-latex/internal/term"
	"github.com/MarkAureli/easy-latex/internal/texscan"
	"github.com/spf13/cobra"
)

var openAfter bool

var compileCmd = &cobra.Command{
	Use:               "compile",
	Short:             "Compile the LaTeX document",
	SilenceUsage:      true,
	RunE:              runCompile,
	ValidArgsFunction: cobra.NoFileCompletions,
}

func init() {
	compileCmd.Flags().BoolVarP(&openAfter, "open", "o", false, "Open PDF after successful compilation")
}

var errorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^!`),
	regexp.MustCompile(`(?i)\berrors?\b`),
}

var warningPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)warning`),
	regexp.MustCompile(`(?i)undefined`),
	regexp.MustCompile(`(?i)multiply defined`),
	regexp.MustCompile(`^(?:Over|Under)full`),
}

var contextLinePattern = regexp.MustCompile(`^l\.\d+`)

var compileColors = term.Detect()

func lineType(line string) string {
	for _, pat := range errorPatterns {
		if pat.MatchString(line) {
			return "error"
		}
	}
	return "warning"
}

func isContextLine(line string) bool {
	return contextLinePattern.MatchString(line)
}

func runCompile(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if _, err := os.Stat(cfg.Main); err != nil {
		return fmt.Errorf("main file %q not found. Re-run 'el init'", cfg.Main)
	}

	if err := os.MkdirAll(auxDir, 0755); err != nil {
		return fmt.Errorf("cannot create %s: %w", auxDir, err)
	}

	pdflatex, err := findTool("pdflatex")
	if err != nil {
		return err
	}

	stem := filepath.Base(strings.TrimSuffix(cfg.Main, ".tex"))

	log := newBibLogger()

	// If bibliography.bib changed since the last compile or bib parse run,
	// auto-allocate new cache entries and record any renames before compiling.
	if ef := entriesBibFile(cfg.BibFiles); ef != "" && bib.BibFileChanged(ef, auxDir) {
		log.Info("", "bibliography.bib changed, re-parsing...")
		added, renames, err := bib.AllocateCacheEntries(cfg.BibFiles, auxDir, cfg.retryTimeout(), log)
		if err != nil {
			return err
		}
		for old, new := range renames {
			log.Info("", fmt.Sprintf("key renamed: %s -> %s", old, new))
		}
		if added > 0 {
			log.Info("", fmt.Sprintf("allocated %d new cache entries", added))
		}
		bib.SaveRenames(auxDir, renames)
		bib.UpdateBibHash(ef, auxDir)
	}

	// First pdflatex pass — buffer output; only print if no bib tool runs,
	// since bib-related warnings (undefined citations, references) are expected
	// at this stage and will be resolved by the subsequent bib tool pass.
	//
	// If the pass fails and a stale .bbl exists (e.g. from a previous failed
	// compile with malformed bib content), delete it and retry once: the .bbl
	// will be regenerated correctly by the bib tool on this run.
	firstLines, err := runPdflatex(pdflatex, cfg)
	if err != nil {
		bblPath := filepath.Join(auxDir, stem+".bbl")
		if _, statErr := os.Stat(bblPath); statErr == nil {
			os.Remove(bblPath) //nolint:errcheck
			firstLines, err = runPdflatex(pdflatex, cfg)
		}
		if err != nil {
			printLines(firstLines)
			return err
		}
	}

	// Update bib file list from artifacts if not already set by el init.
	if len(cfg.BibFiles) == 0 {
		if found := bibFilesFromArtifacts(stem, auxDir); len(found) > 0 {
			cfg.BibFiles = found
			_ = saveConfig(cfg)
		}
	}

	// Apply pending cite-key renames produced by el bib parse: update \cite{}
	// references in all .tex files and re-run pdflatex so the .aux/.bcf reflects
	// the new keys before the bib tool.
	if renames := bib.LoadRenames(auxDir); len(renames) > 0 {
		log.Info("", "rewriting cite keys in .tex files")
		for old, new := range renames {
			log.Info("", fmt.Sprintf("  %s -> %s", old, new))
		}
		texFiles := texscan.FindTexFiles(cfg.Main, ".")
		if err := rewriteCiteKeys(texFiles, renames); err != nil {
			return err
		}
		bib.ClearRenames(auxDir)
		if _, err := runPdflatex(pdflatex, cfg); err != nil {
			return err
		}
	}

	// Generate bibliography.bib from cache for cited entries only.
	if ef := entriesBibFile(cfg.BibFiles); ef != "" {
		citeKeys := citedKeysFromArtifacts(stem, auxDir)
		if len(citeKeys) > 0 {
			if err := bib.WriteBibFromCache(ef, citeKeys, auxDir, bib.WriteOptions{
				AbbreviateJournals:  cfg.abbreviateJournals(),
				BraceTitles:         cfg.braceTitles(),
				IEEEFormat:          cfg.ieeeFormat(),
				MaxAuthors:          cfg.maxAuthors(),
				AbbreviateFirstName: cfg.abbreviateFirstName(),
				UrlFromDOI:          cfg.urlFromDOI(),
			}); err != nil {
				return err
			}
			bib.UpdateBibHash(ef, auxDir)
		}
	}

	// Detect and run bibliography tool based on artifacts from first pass
	bibTool, err := detectBibTool(stem, auxDir)
	if err != nil {
		return err
	}
	if bibTool != "" {
		if err := runBibTool(bibTool, stem, auxDir); err != nil {
			return err
		}
		fixBblBurl(filepath.Join(auxDir, stem+".bbl"))
		// Second pdflatex pass to incorporate bibliography
		secondLines, err := runPdflatex(pdflatex, cfg)
		if err != nil {
			printLines(secondLines)
			return err
		}
		// Additional passes to stabilize cross-references and citations
		prev := secondLines
		for range 2 {
			if !needsRerun(prev) {
				break
			}
			prev, err = runPdflatex(pdflatex, cfg)
			if err != nil {
				printLines(prev)
				return err
			}
		}
		printLines(prev)
	} else {
		printLines(firstLines)
	}

	pdfName := stem + ".pdf"
	srcPDF := filepath.Join(auxDir, pdfName)

	// Copy compiled PDF from aux dir to working dir
	_ = os.Remove(pdfName)
	srcData, err := os.ReadFile(srcPDF)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", srcPDF, err)
	}
	if err := os.WriteFile(pdfName, srcData, 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", pdfName, err)
	}

	fmt.Printf("Compiled successfully -> %s\n", pdfName)

	if openAfter {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", pdfName)
		case "linux":
			cmd = exec.Command("xdg-open", pdfName)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", pdfName)
		}
		if cmd != nil {
			cmd.Start() //nolint:errcheck
		}
	}

	return nil
}

var rerunPattern = regexp.MustCompile(`(?i)rerun`)

func needsRerun(lines []string) bool {
	return slices.ContainsFunc(lines, func(line string) bool {
		return rerunPattern.MatchString(line)
	})
}

func filterLines(output []byte) []string {
	var errors, warnings []string
	lastKind := "" // "error" or "warning"
	for line := range strings.SplitSeq(string(output), "\n") {
		// Context lines (l.123 ...) inherit the category of the preceding diagnostic.
		if contextLinePattern.MatchString(line) {
			switch lastKind {
			case "error":
				errors = append(errors, line)
			case "warning":
				warnings = append(warnings, line)
			}
			continue
		}
		matched := false
		for _, pat := range errorPatterns {
			if pat.MatchString(line) {
				errors = append(errors, line)
				lastKind = "error"
				matched = true
				break
			}
		}
		if !matched {
			for _, pat := range warningPatterns {
				if pat.MatchString(line) {
					warnings = append(warnings, line)
					lastKind = "warning"
					break
				}
			}
		}
	}
	if len(errors) > 0 {
		return errors
	}
	return warnings
}

func printLines(lines []string) {
	if len(lines) == 0 {
		return
	}

	typ := lineType(lines[0])

	if typ == "error" {
		fmt.Printf("%s%sError:%s\n", compileColors.Bold, compileColors.Red, compileColors.Reset)
	} else {
		fmt.Printf("%s%sWarnings:%s\n", compileColors.Bold, compileColors.Yellow, compileColors.Reset)
	}

	color := compileColors.Red
	if typ == "warning" {
		color = compileColors.Yellow
	}

	for i, line := range lines {
		if i > 0 && !isContextLine(line) {
			fmt.Println()
		}
		fmt.Printf("  %s%s%s\n", color, line, compileColors.Reset)
	}
}

func runPdflatex(pdflatex string, cfg *Config) ([]string, error) {
	c := exec.Command(pdflatex,
		"-interaction=nonstopmode",
		"-halt-on-error",
		"-file-line-error",
		"-output-directory="+auxDir,
		cfg.Main,
	)
	output, runErr := c.CombinedOutput()
	lines := filterLines(output)
	if runErr != nil {
		return lines, fmt.Errorf("compilation failed")
	}
	return lines, nil
}

// detectBibTool inspects the aux directory after a pdflatex pass to determine
// which bibliography tool is needed, if any.
// - .bcf file present → biber (written by biblatex regardless of bib source)
// - .aux contains \bibdata → bibtex (written by traditional \bibliography{})
// - neither → no bibliography step needed
func detectBibTool(stem, auxDir string) (string, error) {
	bcf := filepath.Join(auxDir, stem+".bcf")
	if _, err := os.Stat(bcf); err == nil {
		return "biber", nil
	}

	auxFile := filepath.Join(auxDir, stem+".aux")
	data, err := os.ReadFile(auxFile)
	if err != nil {
		return "", nil // no aux file, no bibliography
	}
	if strings.Contains(string(data), `\bibdata{`) {
		return "bibtex", nil
	}

	return "", nil
}

func runBibTool(tool, stem, auxDir string) error {
	toolPath, err := findTool(tool)
	if err != nil {
		return err
	}

	var c *exec.Cmd
	if tool == "biber" {
		// biber takes the stem; --input/output-directory tell it where the .bcf and .bbl live
		c = exec.Command(toolPath, "--input-directory="+auxDir, "--output-directory="+auxDir, stem)
	} else {
		// bibtex runs from inside the aux dir (so its output files aren't written
		// through a dot-path, which TeX Live's openout_any=p security policy blocks).
		// BIBINPUTS=..: tells bibtex to look for .bib files in the project root first.
		c = exec.Command(toolPath, stem)
		c.Dir = auxDir
		c.Env = append(os.Environ(), "BIBINPUTS=..:", "BSTINPUTS=..:")
	}

	output, runErr := c.CombinedOutput()
	printLines(filterLines(output))
	if runErr != nil {
		return fmt.Errorf("%s failed", tool)
	}
	return nil
}

// fixBblBurl patches a .bbl file so the fallback \burl definition uses \url
// instead of \textsf.  \textsf does not handle special URL characters (_#%…),
// causing "Missing $ inserted" errors.
func fixBblBurl(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	const old = `\def \burl#1{\textsf{#1}}`
	const new_ = `\def \burl#1{\url{#1}}`
	s := string(data)
	if !strings.Contains(s, old) {
		return
	}
	s = strings.Replace(s, old, new_, 1)
	os.WriteFile(path, []byte(s), 0644) //nolint:errcheck
}

func findTool(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	// macOS — MacTeX
	if fallback := "/Library/TeX/texbin/" + name; statExists(fallback) {
		return fallback, nil
	}
	// Linux — TeX Live (year and arch vary)
	matches, _ := filepath.Glob("/usr/local/texlive/*/bin/*/" + name)
	if len(matches) > 0 {
		return matches[len(matches)-1], nil // latest year
	}
	return "", fmt.Errorf("%s not found in PATH or common TeX Live installation directories. Install TeX Live or MacTeX", name)
}

func statExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var (
	reBibData       = regexp.MustCompile(`\\bibdata\{([^}]+)\}`)
	reBcfDatasource = regexp.MustCompile(`<bcf:datasource[^>]*datatype="bibtex"[^>]*>([^<]+)</bcf:datasource>`)
	reAuxCitation   = regexp.MustCompile(`\\citation\{([^}]+)\}`)
	reBcfCitekey    = regexp.MustCompile(`<bcf:citekey[^>]*>([^<]+)</bcf:citekey>`)
)

// bibFilesFromArtifacts discovers .bib file names from the .aux and .bcf
// artifacts produced by pdflatex. Returns paths relative to the project root.
func bibFilesFromArtifacts(stem, auxDir string) []string {
	seen := map[string]bool{}
	var files []string

	add := func(name string) {
		if !strings.HasSuffix(name, ".bib") {
			name += ".bib"
		}
		if !seen[name] {
			seen[name] = true
			files = append(files, name)
		}
	}

	if data, err := os.ReadFile(filepath.Join(auxDir, stem+".aux")); err == nil {
		for _, m := range reBibData.FindAllStringSubmatch(string(data), -1) {
			for name := range strings.SplitSeq(m[1], ",") {
				add(strings.TrimSpace(name))
			}
		}
	}

	if data, err := os.ReadFile(filepath.Join(auxDir, stem+".bcf")); err == nil {
		for _, m := range reBcfDatasource.FindAllStringSubmatch(string(data), -1) {
			add(strings.TrimSpace(m[1]))
		}
	}

	return files
}

// citedKeysFromArtifacts extracts the set of citation keys from the .aux and
// .bcf artifacts produced by pdflatex, preserving first-seen order.
func citedKeysFromArtifacts(stem, auxDir string) []string {
	seen := map[string]bool{}
	var keys []string
	add := func(key string) {
		key = strings.TrimSpace(key)
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	if data, err := os.ReadFile(filepath.Join(auxDir, stem+".aux")); err == nil {
		for _, m := range reAuxCitation.FindAllStringSubmatch(string(data), -1) {
			for k := range strings.SplitSeq(m[1], ",") {
				add(k)
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(auxDir, stem+".bcf")); err == nil {
		for _, m := range reBcfCitekey.FindAllStringSubmatch(string(data), -1) {
			add(m[1])
		}
	}
	return keys
}

// rewriteCiteKeys replaces old citation keys with their renamed counterparts in
// all given .tex files. Only occurrences delimited by {, } or , (with optional
// surrounding whitespace) are replaced, avoiding false matches in prose or labels.
func rewriteCiteKeys(texFiles []string, renames map[string]string) error {
	type rule struct {
		re  *regexp.Regexp
		new string
	}
	rules := make([]rule, 0, len(renames))
	for old, newKey := range renames {
		pat := regexp.MustCompile(`([{,]\s*)` + regexp.QuoteMeta(old) + `(\s*[},])`)
		rules = append(rules, rule{pat, newKey})
	}

	for _, path := range texFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", path, err)
		}
		lines := strings.Split(string(data), "\n")
		changed := false
		for i, line := range lines {
			// Only rewrite the non-comment portion of each line.
			commentIdx := strings.Index(line, "%")
			pre, suf := line, ""
			if commentIdx >= 0 {
				pre, suf = line[:commentIdx], line[commentIdx:]
			}
			newPre := pre
			for _, r := range rules {
				newPre = r.re.ReplaceAllString(newPre, "${1}"+r.new+"${2}")
			}
			if newPre != pre {
				lines[i] = newPre + suf
				changed = true
			}
		}
		if changed {
			if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
				return fmt.Errorf("cannot write %s: %w", path, err)
			}
		}
	}
	return nil
}
