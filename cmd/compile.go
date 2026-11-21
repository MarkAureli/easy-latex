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
	"github.com/MarkAureli/easy-latex/internal/cmdprobe"
	"github.com/MarkAureli/easy-latex/internal/envprobe"
	"github.com/MarkAureli/easy-latex/internal/pedantic"
	"github.com/MarkAureli/easy-latex/internal/term"
	"github.com/MarkAureli/easy-latex/internal/texscan"
	"github.com/spf13/cobra"
)

var (
	openAfter      bool
	compileFix     bool
	compileNoCheck bool
	compileEngine  string
)

var compileCmd = &cobra.Command{
	Use:   "compile",
	Short: "Compile the LaTeX document",
	Long: `Compile the LaTeX document (multiple pdflatex passes, bib tools as needed).

Runs pedantic checks after compilation if enabled in config.`,
	SilenceUsage:      true,
	RunE:              runCompile,
	ValidArgsFunction: cobra.NoFileCompletions,
}

func init() {
	compileCmd.Flags().BoolVarP(&openAfter, "open", "o", false, "Open PDF after successful compilation")
	compileCmd.Flags().BoolVarP(&compileFix, "fix", "f", false, "Apply autofixes to source files where available")
	compileCmd.Flags().BoolVarP(&compileNoCheck, "no-check", "n", false, "Skip all pedantic checks (static and dynamic)")
	compileCmd.Flags().StringVar(&compileEngine, "engine", "", "LaTeX engine: pdflatex, xelatex, or lualatex (overrides config and magic comment)")
}

// magicEnginePattern matches the de-facto TeXShop/VSCode convention:
//
//	% !TEX program = xelatex
//	% !TEX TS-program = lualatex
var magicEnginePattern = regexp.MustCompile(`(?i)^\s*%\s*!TEX\s+(?:TS-)?program\s*=\s*(\w+)`)

// detectMagicEngine scans the first ~20 lines of mainTex for a magic comment.
// Returns the engine name in lowercase, or "" if none found.
func detectMagicEngine(mainTex string) string {
	f, err := os.Open(mainTex)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	lines := strings.SplitN(string(buf[:n]), "\n", 21)
	if len(lines) > 20 {
		lines = lines[:20]
	}
	for _, line := range lines {
		if m := magicEnginePattern.FindStringSubmatch(line); m != nil {
			return strings.ToLower(m[1])
		}
	}
	return ""
}

// resolveEngine picks the LaTeX engine per precedence:
// CLI flag → magic comment → config → "pdflatex".
func resolveEngine(cfg *Config, flagVal string) (string, error) {
	if flagVal != "" {
		if err := validateEngine(flagVal); err != nil {
			return "", err
		}
		return flagVal, nil
	}
	if magic := detectMagicEngine(cfg.Main); magic != "" {
		if err := validateEngine(magic); err != nil {
			return "", fmt.Errorf("magic comment in %s: %w", cfg.Main, err)
		}
		return magic, nil
	}
	engine := cfg.engine()
	if err := validateEngine(engine); err != nil {
		return "", err
	}
	return engine, nil
}

var compileColors = term.Detect()

func runCompile(cmd *cobra.Command, args []string) error {
	if compileFix && compileNoCheck {
		return fmt.Errorf("--fix and --no-check are mutually exclusive")
	}
	r, err := prepareCompile()
	if err != nil {
		return err
	}
	if err := r.syncBibCache(); err != nil {
		return err
	}
	diags, err := r.runPasses()
	if err != nil {
		return err
	}

	// Best effort: learn environment and command signatures for editor completion.
	_ = updateEnvTable(r.stem, r.scannedEnvs)
	_ = updateCmdTable(r.stem, r.cfg.Main)

	pdfName, err := r.copyPDF()
	if err != nil {
		return err
	}

	var rep checkReport
	if r.runChecks {
		texFiles := texscan.FindTexFiles(r.cfg.Main, ".")
		if rep, err = runSourceChecks(r.cfg, r.checks, texFiles, compileFix); err != nil {
			return err
		}
		rep.ped = append(rep.ped, pedantic.RunPostCompileChecks(r.checks, auxDir)...)
		sortDiagnostics(rep.ped)
	}

	// Best effort: expose diagnostics to editors (VimTeX) via the engine log.
	_ = appendDiagsToLog(filepath.Join(auxDir, r.stem+".log"), r.cfg.Main, rep.ped, rep.spell)

	printReport(rep, ofKind(diags, logWarning), compileFix, true)
	fmt.Printf("Compiled successfully -> %s\n", pdfName)
	if openAfter {
		openFile(pdfName)
	}
	return nil
}

// compileRun carries the state shared by the stages of one el compile.
type compileRun struct {
	cfg         *Config
	stem        string // main file name without .tex; the engine's jobname
	enginePath  string
	injectStys  []string // packages \RequirePackage'd before the document
	scannedEnvs map[string]texscan.EnvInfo
	checks      []string // enabled pedantic checks
	runChecks   bool
	log         bib.Logger
}

// prepareCompile loads the config, resolves the engine and writes the
// instrumentation packages (pedantic post-compile checks, env probe).
func prepareCompile() (*compileRun, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	r := &compileRun{
		cfg:    cfg,
		stem:   filepath.Base(strings.TrimSuffix(cfg.Main, ".tex")),
		checks: cfg.Pedantic.EnabledNames(),
		log:    newBibLogger(),
	}
	r.runChecks = !compileNoCheck && (len(r.checks) > 0 || cfg.Spelling != nil)
	if r.runChecks {
		if err := pedantic.ValidateCheckNames(r.checks); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(cfg.Main); err != nil {
		return nil, fmt.Errorf("main file %q not found. Re-run 'el init'", cfg.Main)
	}
	if err := os.MkdirAll(auxDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create %s: %w", auxDir, err)
	}

	if !compileNoCheck {
		for name, data := range pedantic.PostCompileStys(r.checks) {
			styPath := filepath.Join(auxDir, name)
			if err := os.WriteFile(styPath, data, 0644); err != nil {
				return nil, fmt.Errorf("cannot write %s: %w", styPath, err)
			}
			r.injectStys = append(r.injectStys, strings.TrimSuffix(name, ".sty"))
		}
	}
	// Instrument the compile to learn environment kinds (see envprobe).
	r.scannedEnvs = scanProjectEnvs(cfg.Main)
	if err := writeEnvProbe(r.scannedEnvs); err != nil {
		return nil, fmt.Errorf("cannot write env probe: %w", err)
	}
	r.injectStys = append(r.injectStys, envprobe.PkgName)
	// Instrument the compile to learn command signatures (see cmdprobe).
	if err := writeCmdProbe(cfg.Main); err != nil {
		return nil, fmt.Errorf("cannot write cmd probe: %w", err)
	}
	r.injectStys = append(r.injectStys, cmdprobe.PkgName)
	slices.Sort(r.injectStys)

	engine, err := resolveEngine(cfg, compileEngine)
	if err != nil {
		return nil, err
	}
	if r.enginePath, err = findTool(engine); err != nil {
		return nil, err
	}
	return r, nil
}

// syncBibCache re-parses bibliography.bib into the global cache if it changed
// since the last compile or bib parse, recording renames for applyRenames.
func (r *compileRun) syncBibCache() error {
	ef := entriesBibFile(r.cfg.BibFiles)
	if ef == "" || !bib.BibFileChanged(ef, auxDir) {
		return nil
	}
	r.log.Info("", "bibliography.bib changed, re-parsing...")
	added, renames, err := bib.AllocateCacheEntries(r.cfg.BibFiles, r.cfg.retryTimeout(), r.log)
	if err != nil {
		return err
	}
	for old, new := range renames {
		r.log.Info("", fmt.Sprintf("key renamed: %s -> %s", old, new))
	}
	if added > 0 {
		r.log.Info("", fmt.Sprintf("allocated %d new cache entries", added))
	}
	bib.SaveRenames(auxDir, renames)
	bib.UpdateBibHash(ef, auxDir)
	return nil
}

// pass runs one engine pass and prints its errors if it fails.
func (r *compileRun) pass(draft bool) ([]logDiag, error) {
	diags, err := runEngine(r.enginePath, r.cfg, r.injectStys, draft)
	if err != nil {
		printLogDiags(os.Stdout, diags)
	}
	return diags, err
}

// runPasses runs the full pass sequence and returns the diagnostics of the
// final, PDF-writing pass:
//
//	engine → [cite-key renames → engine] → bib from cache → [bib tool →
//	engine (+ up to 2 reruns)] → final engine pass
//
// All passes but the last run with -draftmode, so PDF viewers only reload
// once. Bib-related warnings of earlier passes are expected and not shown.
func (r *compileRun) runPasses() ([]logDiag, error) {
	// A stale .bbl (e.g. from a previous compile with malformed bib content)
	// can break the first pass: delete it and retry once; the bib tool
	// regenerates it below.
	if diags, err := runEngine(r.enginePath, r.cfg, r.injectStys, true); err != nil {
		bblPath := filepath.Join(auxDir, r.stem+".bbl")
		if _, statErr := os.Stat(bblPath); statErr == nil {
			os.Remove(bblPath) //nolint:errcheck
			diags, err = runEngine(r.enginePath, r.cfg, r.injectStys, true)
		}
		if err != nil {
			printLogDiags(os.Stdout, diags)
			return nil, err
		}
	}
	r.discoverBibFiles()
	if err := r.applyRenames(); err != nil {
		return nil, err
	}
	if err := r.writeBib(); err != nil {
		return nil, err
	}

	bibTool, err := detectBibTool(r.stem, auxDir)
	if err != nil {
		return nil, err
	}
	if bibTool != "" {
		if err := runBibTool(bibTool, r.stem, auxDir); err != nil {
			return nil, err
		}
		fixBblBurl(filepath.Join(auxDir, r.stem+".bbl"))
		diags, err := r.pass(true)
		// Additional passes to stabilize cross-references and citations.
		for i := 0; err == nil && i < 2 && needsRerun(diags); i++ {
			diags, err = r.pass(true)
		}
		if err != nil {
			return nil, err
		}
	}
	return r.pass(false)
}

// discoverBibFiles records the bib files the first pass used in the local
// config, if el init did not set any.
func (r *compileRun) discoverBibFiles() {
	if len(r.cfg.BibFiles) > 0 {
		return
	}
	if found := bibFilesFromArtifacts(r.stem, auxDir); len(found) > 0 {
		r.cfg.BibFiles = found
		localCfg, _ := loadLocalConfig()
		localCfg.BibFiles = found
		_ = saveLocalConfig(localCfg)
	}
}

// applyRenames rewrites \cite{} keys renamed by a bib parse in all .tex files
// and reruns the engine so the .aux/.bcf carry the new keys.
func (r *compileRun) applyRenames() error {
	renames := bib.LoadRenames(auxDir)
	if len(renames) == 0 {
		return nil
	}
	r.log.Info("", "rewriting cite keys in .tex files")
	for old, new := range renames {
		r.log.Info("", fmt.Sprintf("  %s -> %s", old, new))
	}
	if err := rewriteCiteKeys(texscan.FindTexFiles(r.cfg.Main, "."), renames); err != nil {
		return err
	}
	bib.ClearRenames(auxDir)
	_, err := r.pass(true)
	return err
}

// writeBib generates bibliography.bib from the cache for the cited keys.
func (r *compileRun) writeBib() error {
	ef := entriesBibFile(r.cfg.BibFiles)
	if ef == "" {
		return nil
	}
	// Drop keys defined in generated .bib files (e.g. revtex's citeautoscript
	// writes mainNotes.bib with @FOOTNOTE{Note1,...}): they live outside our
	// cache and bibtex resolves them on its own.
	excluded := generatedBibKeys(r.stem, auxDir, r.cfg.BibFiles)
	citeKeys := slices.DeleteFunc(citedKeysFromArtifacts(r.stem, auxDir), func(k string) bool { return excluded[k] })
	if len(citeKeys) == 0 {
		return nil
	}
	cfg := r.cfg
	if err := bib.WriteBibFromCache(ef, citeKeys, bib.WriteOptions{
		AbbreviateJournals:  cfg.abbreviateJournals(),
		BraceTitles:         cfg.braceTitles(),
		ArxivAsUnpublished:  cfg.arxivAsUnpublished(),
		MaxAuthors:          cfg.maxAuthors(),
		AbbreviateFirstName: cfg.abbreviateFirstName(),
		UrlFromDOI:          cfg.urlFromDOI(),
	}); err != nil {
		return err
	}
	bib.UpdateBibHash(ef, auxDir)
	return nil
}

// copyPDF copies the compiled PDF from the aux dir to the working dir and
// returns its name.
func (r *compileRun) copyPDF() (string, error) {
	pdfName := r.stem + ".pdf"
	srcPDF := filepath.Join(auxDir, pdfName)
	_ = os.Remove(pdfName)
	data, err := os.ReadFile(srcPDF)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", srcPDF, err)
	}
	if err := os.WriteFile(pdfName, data, 0644); err != nil {
		return "", fmt.Errorf("cannot write %s: %w", pdfName, err)
	}
	return pdfName, nil
}

// openFile opens path with the platform's default viewer, without waiting.
func openFile(path string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", path)
	case "linux":
		c = exec.Command("xdg-open", path)
	case "windows":
		c = exec.Command("cmd", "/c", "start", path)
	}
	if c != nil {
		c.Start() //nolint:errcheck
	}
}

func runEngine(enginePath string, cfg *Config, injectStys []string, draft bool) ([]logDiag, error) {
	args := []string{
		"-synctex=1",
		"-interaction=nonstopmode",
		"-halt-on-error",
		"-file-line-error",
		"-output-directory=" + auxDir,
	}
	if draft {
		args = append(args, "-draftmode")
	}
	input := cfg.Main
	if len(injectStys) > 0 {
		// Wrap input to inject pedantic post-compile .sty packages before the document.
		stem := strings.TrimSuffix(filepath.Base(cfg.Main), ".tex")
		args = append(args, "-jobname="+stem)
		var reqs strings.Builder
		for _, pkg := range injectStys {
			reqs.WriteString(`\RequirePackage{` + pkg + `}`)
		}
		input = reqs.String() + `\input{` + cfg.Main + `}`
	}
	args = append(args, input)
	c := exec.Command(enginePath, args...)
	// Avoid hard-wrapping log lines at 79 columns so warnings (and their
	// "on input line N") stay on one line for editors parsing the log (VimTeX).
	c.Env = append(os.Environ(), "max_print_line=1000", "error_line=79", "half_error_line=50")
	if len(injectStys) > 0 {
		// Add aux dir to TEXINPUTS so the engine finds the injected .sty files.
		absAux, _ := filepath.Abs(auxDir)
		c.Env = append(c.Env, "TEXINPUTS="+absAux+string(os.PathListSeparator))
	}
	output, runErr := c.CombinedOutput()
	diags := parseLog(output)
	if runErr != nil {
		return diags, fmt.Errorf("compilation failed")
	}
	return diags, nil
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
	printLogDiags(os.Stdout, parseLog(output))
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

// generatedBibKeys returns cite keys defined in .bib files referenced by
// \bibdata{...} in the .aux but not declared in cfg.BibFiles. Such files are
// produced by LaTeX packages — e.g. revtex's citeautoscript writes
// <stem>Notes.bib with @FOOTNOTE{NoteN,...} entries for each \footnote.
// Generated files are searched in auxDir first, then the project root.
func generatedBibKeys(stem, auxDir string, userBibFiles []string) map[string]bool {
	user := map[string]bool{}
	for _, f := range userBibFiles {
		user[filepath.Base(f)] = true
	}
	keys := map[string]bool{}
	for _, name := range bibFilesFromArtifacts(stem, auxDir) {
		if user[filepath.Base(name)] {
			continue
		}
		var data []byte
		for _, candidate := range []string{filepath.Join(auxDir, name), name} {
			if b, err := os.ReadFile(candidate); err == nil {
				data = b
				break
			}
		}
		if data == nil {
			continue
		}
		for _, item := range bib.ParseFile(string(data)) {
			if item.IsEntry && item.Entry.Key != "" {
				keys[item.Entry.Key] = true
			}
		}
	}
	return keys
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
			// StripComment honours escaped \% so prose like "92\,\%" is not
			// mistaken for a comment start.
			pre := texscan.StripComment(line)
			suf := line[len(pre):]
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
