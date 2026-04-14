package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/bib"
	"github.com/MarkAureli/easy-latex/internal/texscan"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize easy-latex in the current directory",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	return doInit(".", os.Stdin)
}

func doInit(dir string, stdin io.Reader) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read current directory: %w", err)
	}

	var matches []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tex") {
			continue
		}
		if hasBeginDocument(filepath.Join(dir, e.Name())) {
			matches = append(matches, e.Name())
		}
	}

	var chosen string
	switch len(matches) {
	case 0:
		return fmt.Errorf("no .tex files with \\begin{document} found in current directory")
	case 1:
		chosen = matches[0]
	default:
		chosen, err = pickFile(matches, stdin)
		if err != nil {
			return err
		}
	}

	elDir := filepath.Join(dir, ".el")
	if err := os.MkdirAll(elDir, 0755); err != nil {
		return fmt.Errorf("cannot create .el: %w", err)
	}

	if err := texscan.ResolveFileContents(chosen, dir); err != nil {
		return err
	}

	bibFiles := texscan.FindBibFiles(chosen, dir)

	cfg := Config{Main: chosen, BibFiles: bibFiles}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(elDir, "config.json"), data, 0644); err != nil {
		return fmt.Errorf("cannot write .el/config.json: %w", err)
	}

	if err := updateGitExclude(dir); err != nil {
		return err
	}

	if _, err := bib.AllocateCacheEntries(bibFiles, elDir); err != nil {
		return err
	}

	fmt.Printf("Initialized. Main file: %s\n", chosen)
	if len(bibFiles) > 0 {
		fmt.Printf("Bib files: %s\n", strings.Join(bibFiles, ", "))
	}
	return nil
}

// updateGitExclude appends .el to .git/info/exclude if a
// .git directory is present and the entry is not already listed.
func updateGitExclude(dir string) error {
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return nil // not a git repo, nothing to do
	}

	excludePath := filepath.Join(gitDir, "info", "exclude")

	// Read existing entries to avoid duplicates
	existing := map[string]bool{}
	data, readErr := os.ReadFile(excludePath)
	if readErr == nil {
		for _, line := range strings.Split(string(data), "\n") {
			existing[strings.TrimSpace(line)] = true
		}
	}

	var toAdd []string
	for _, entry := range []string{".el"} {
		if !existing[entry] {
			toAdd = append(toAdd, entry)
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Join(gitDir, "info"), 0755); err != nil {
		return fmt.Errorf("cannot create .git/info: %w", err)
	}

	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open .git/info/exclude: %w", err)
	}
	defer f.Close()

	// Ensure we start on a fresh line if the file already has content
	if readErr == nil && len(data) > 0 && data[len(data)-1] != '\n' {
		fmt.Fprintln(f)
	}
	for _, entry := range toAdd {
		fmt.Fprintln(f, entry)
	}

	return nil
}

func hasBeginDocument(filename string) bool {
	f, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), `\begin{document}`) {
			return true
		}
	}
	return false
}

func pickFile(files []string, stdin io.Reader) (string, error) {
	fmt.Println("Found multiple .tex files with \\begin{document}:")
	for i, f := range files {
		fmt.Printf("  [%d] %s\n", i+1, f)
	}

	reader := bufio.NewReader(stdin)
	for {
		fmt.Printf("Enter number (1-%d): ", len(files))
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}
		line = strings.TrimSpace(line)
		var n int
		if _, err := fmt.Sscanf(line, "%d", &n); err == nil && n >= 1 && n <= len(files) {
			return files[n-1], nil
		}
		fmt.Fprintf(os.Stderr, "Invalid choice. Please enter a number between 1 and %d.\n", len(files))
	}
}
