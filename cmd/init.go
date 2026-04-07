package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize easy-latex in the current directory",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	entries, err := os.ReadDir(".")
	if err != nil {
		return fmt.Errorf("cannot read current directory: %w", err)
	}

	var matches []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tex") {
			continue
		}
		if hasBeginDocument(e.Name()) {
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
		chosen, err = pickFile(matches)
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(".aux_dir", 0755); err != nil {
		return fmt.Errorf("cannot create .aux_dir: %w", err)
	}

	cfg := Config{Main: chosen, AuxDir: ".aux_dir"}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(".el.json", data, 0644); err != nil {
		return fmt.Errorf("cannot write .el.json: %w", err)
	}

	fmt.Printf("Initialized. Main file: %s\n", chosen)
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

func pickFile(files []string) (string, error) {
	fmt.Println("Found multiple .tex files with \\begin{document}:")
	for i, f := range files {
		fmt.Printf("  [%d] %s\n", i+1, f)
	}

	reader := bufio.NewReader(os.Stdin)
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
