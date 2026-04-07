package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var openAfter bool

var compileCmd = &cobra.Command{
	Use:   "compile",
	Short: "Compile the LaTeX document",
	RunE:  runCompile,
}

func init() {
	compileCmd.Flags().BoolVarP(&openAfter, "open", "o", false, "Open PDF after successful compilation")
}

var keepPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^!`),
	regexp.MustCompile(`^l\.\d+`),
	regexp.MustCompile(`(?i)warning`),
	regexp.MustCompile(`(?i)error`),
	regexp.MustCompile(`(?i)undefined`),
	regexp.MustCompile(`(?i)multiply defined`),
	regexp.MustCompile(`^(?:Over|Under)full`),
}

func runCompile(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if _, err := os.Stat(cfg.Main); err != nil {
		return fmt.Errorf("main file %q not found. Re-run 'el init'", cfg.Main)
	}

	if err := os.MkdirAll(cfg.AuxDir, 0755); err != nil {
		return fmt.Errorf("cannot create %s: %w", cfg.AuxDir, err)
	}

	pdflatex, err := findPdflatex()
	if err != nil {
		return err
	}

	stem := filepath.Base(strings.TrimSuffix(cfg.Main, ".tex"))

	// First pdflatex pass
	if err := runPdflatex(pdflatex, cfg); err != nil {
		return err
	}

	// Detect and run bibliography tool based on artifacts from first pass
	bibTool, err := detectBibTool(stem, cfg.AuxDir)
	if err != nil {
		return err
	}
	if bibTool != "" {
		if err := runBibTool(bibTool, stem, cfg.AuxDir); err != nil {
			return err
		}
		// Second pdflatex pass to incorporate bibliography
		if err := runPdflatex(pdflatex, cfg); err != nil {
			return err
		}
	}

	pdfName := stem + ".pdf"
	srcPDF := filepath.Join(cfg.AuxDir, pdfName)

	// Remove stale symlink or file, then create symlink
	_ = os.Remove(pdfName)
	if err := os.Symlink(srcPDF, pdfName); err != nil {
		return fmt.Errorf("cannot create symlink for %s: %w", pdfName, err)
	}

	fmt.Printf("Compiled successfully -> %s\n", pdfName)

	if openAfter {
		exec.Command("open", pdfName).Start() //nolint:errcheck
	}

	return nil
}

func runPdflatex(pdflatex string, cfg *Config) error {
	c := exec.Command(pdflatex,
		"-interaction=nonstopmode",
		"-file-line-error",
		"-output-directory="+cfg.AuxDir,
		cfg.Main,
	)
	output, runErr := c.CombinedOutput()

	for _, line := range strings.Split(string(output), "\n") {
		for _, pat := range keepPatterns {
			if pat.MatchString(line) {
				fmt.Println(line)
				break
			}
		}
	}

	if runErr != nil {
		return fmt.Errorf("compilation failed")
	}
	return nil
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
		c.Env = append(os.Environ(), "BIBINPUTS=..:")
	}

	output, runErr := c.CombinedOutput()

	for _, line := range strings.Split(string(output), "\n") {
		for _, pat := range keepPatterns {
			if pat.MatchString(line) {
				fmt.Println(line)
				break
			}
		}
	}

	if runErr != nil {
		return fmt.Errorf("%s failed", tool)
	}
	return nil
}

func findTool(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	fallback := "/Library/TeX/texbin/" + name
	if _, err := os.Stat(fallback); err == nil {
		return fallback, nil
	}
	return "", fmt.Errorf("%s not found in PATH or %s. Install TeX Live or MacTeX", name, fallback)
}

func findPdflatex() (string, error) {
	return findTool("pdflatex")
}
