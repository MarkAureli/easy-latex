package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Config is the structure stored in .el.json
type Config struct {
	Main     string   `json:"main"`
	AuxDir   string   `json:"aux_dir"`
	BibFiles []string `json:"bib_files,omitempty"`
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(".el.json")
	if err != nil {
		return nil, fmt.Errorf("not initialized. Run 'el init' first")
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("corrupt .el.json: %w", err)
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(".el.json", data, 0644)
}

var rootCmd = &cobra.Command{
	Use:     "el",
	Short:   "easy-latex: simple LaTeX compilation",
	Version: "0.1.0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(compileCmd)
}
