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
	Use:               "init",
	Short:             "Initialize easy-latex in the current directory",
	RunE:              runInit,
	ValidArgsFunction: cobra.NoFileCompletions,
}

var flagInitIEEE bool

func init() {
	initCmd.Flags().BoolVar(&flagInitIEEE, "ieee", false,
		"Use IEEE bib file names (IEEEabrv.bib, bibliography.bib) and enable IEEE formatting")
}

func runInit(cmd *cobra.Command, args []string) error {
	return doInit(".", os.Stdin, flagInitIEEE)
}

func doInit(dir string, stdin io.Reader, ieee bool) error {
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

	refName := "bibliography.bib"

	var entryBibFiles []string
	if len(bibFiles) > 0 {
		bibFiles, err = condenseBibFiles(bibFiles, dir, ieee)
		if err != nil {
			return err
		}
		if err := texscan.RewriteBibReferences(chosen, dir, bibFiles); err != nil {
			return err
		}
		entryBibFiles = []string{refName}
	}

	cfg := Config{Main: chosen, BibFiles: bibFiles}
	if ieee {
		t := true
		cfg.IEEEFormat = &t
	}
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

	if _, renames, err := bib.AllocateCacheEntries(entryBibFiles, elDir); err != nil {
		return err
	} else if len(renames) > 0 {
		bib.SaveRenames(elDir, renames)
	}
	if len(entryBibFiles) > 0 {
		bib.UpdateBibHash(filepath.Join(dir, refName), elDir)
	}

	fmt.Printf("Initialized. Main file: %s\n", chosen)
	if len(bibFiles) > 0 {
		fmt.Printf("Bib files: %s\n", strings.Join(bibFiles, ", "))
	}
	return nil
}

// condenseBibFiles consolidates all bibFiles into at most two files in dir.
// Entries → bibliography.bib; @string/@preamble → preamble.bib (or IEEEabrv.bib with ieee).
// Original files are deleted. Returns the list of new bib files (preamble first if present).
func condenseBibFiles(bibFiles []string, dir string, ieee bool) ([]string, error) {
	var allEntries []bib.Entry
	var preambleChunks []string

	for _, bibFile := range bibFiles {
		data, err := os.ReadFile(filepath.Join(dir, bibFile))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("cannot read %s: %w", bibFile, err)
		}

		for _, item := range bib.ParseFile(string(data)) {
			if item.IsEntry {
				allEntries = append(allEntries, item.Entry)
				continue
			}
			trimmed := strings.TrimSpace(item.Raw)
			if trimmed == "" {
				continue
			}
			// Drop @comment blocks
			if strings.HasPrefix(strings.ToLower(trimmed), "@comment") {
				continue
			}
			// @string / @preamble blocks: keep as-is
			if trimmed[0] == '@' {
				preambleChunks = append(preambleChunks, trimmed)
				continue
			}
			// Plain text between @-blocks: strip comment lines
			if chunk := filterPreambleText(item.Raw); chunk != "" {
				preambleChunks = append(preambleChunks, chunk)
			}
		}
	}

	refName := "bibliography.bib"
	preName := "preamble.bib"
	if ieee {
		preName = "IEEEabrv.bib"
	}

	refPath := filepath.Join(dir, refName)
	if err := os.WriteFile(refPath, []byte(bib.RenderEntries(allEntries)), 0644); err != nil {
		return nil, fmt.Errorf("cannot write %s: %w", refName, err)
	}

	newBibFiles := []string{refName}

	if len(preambleChunks) > 0 {
		preamblePath := filepath.Join(dir, preName)
		content := strings.Join(preambleChunks, "\n\n") + "\n"
		if err := os.WriteFile(preamblePath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("cannot write %s: %w", preName, err)
		}
		newBibFiles = []string{preName, refName}
	}

	// Delete original files that are not one of the new output files
	newRefAbs, _ := filepath.Abs(refPath)
	newPreAbs, _ := filepath.Abs(filepath.Join(dir, preName))
	for _, bibFile := range bibFiles {
		absPath, _ := filepath.Abs(filepath.Join(dir, bibFile))
		if absPath != newRefAbs && absPath != newPreAbs {
			_ = os.Remove(filepath.Join(dir, bibFile))
		}
	}

	return newBibFiles, nil
}

// filterPreambleText strips comment lines (starting with %) from raw plain-text
// bib chunks and trims leading/trailing blank lines, preserving interior blank lines.
func filterPreambleText(raw string) string {
	lines := strings.Split(raw, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "%") {
			continue
		}
		kept = append(kept, line)
	}
	// Trim leading and trailing blank lines
	start, end := 0, len(kept)-1
	for start <= end && strings.TrimSpace(kept[start]) == "" {
		start++
	}
	for end >= start && strings.TrimSpace(kept[end]) == "" {
		end--
	}
	if start > end {
		return ""
	}
	return strings.Join(kept[start:end+1], "\n")
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
		for line := range strings.SplitSeq(string(data), "\n") {
			existing[strings.TrimSpace(line)] = true
		}
	}

	if existing[".el"] {
		return nil
	}
	toAdd := []string{".el"}

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
