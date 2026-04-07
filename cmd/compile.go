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

	stem := filepath.Base(strings.TrimSuffix(cfg.Main, ".tex"))
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

func findPdflatex() (string, error) {
	if path, err := exec.LookPath("pdflatex"); err == nil {
		return path, nil
	}
	fallback := "/Library/TeX/texbin/pdflatex"
	if _, err := os.Stat(fallback); err == nil {
		return fallback, nil
	}
	return "", fmt.Errorf("pdflatex not found in PATH or %s. Install TeX Live or MacTeX", fallback)
}
