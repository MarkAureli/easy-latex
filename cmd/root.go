package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Config is the structure stored in .el/config.json
type Config struct {
	Main                string   `json:"main"`
	AuxDir              string   `json:"aux_dir"`
	BibFiles            []string `json:"bib_files,omitempty"`
	AbbreviateJournals  *bool    `json:"abbreviate_journals,omitempty"`
	BraceTitles         *bool    `json:"brace_titles,omitempty"`
	IEEEFormat          *bool    `json:"ieee_format,omitempty"`
	MaxAuthors          *int     `json:"max_authors,omitempty"`
	AbbreviateFirstName *bool    `json:"abbreviate_first_name,omitempty"`
}

// abbreviateJournals returns true when journal abbreviation is enabled.
// Defaults to true when the field is absent (nil).
func (cfg *Config) abbreviateJournals() bool {
	return cfg.AbbreviateJournals == nil || *cfg.AbbreviateJournals
}

// braceTitles returns true when title double-bracing is enabled.
// Defaults to false when the field is absent (nil).
func (cfg *Config) braceTitles() bool {
	return cfg.BraceTitles != nil && *cfg.BraceTitles
}

// ieeeFormat returns true when IEEE bib formatting is enabled.
// Defaults to false when the field is absent (nil).
func (cfg *Config) ieeeFormat() bool {
	return cfg.IEEEFormat != nil && *cfg.IEEEFormat
}

// abbreviateFirstName returns true when first (and middle) names should be
// abbreviated to initials. Defaults to true when the field is absent (nil).
func (cfg *Config) abbreviateFirstName() bool {
	return cfg.AbbreviateFirstName == nil || *cfg.AbbreviateFirstName
}

// maxAuthors returns the maximum number of authors to store, or 0 for unlimited.
// Defaults to 0 (unlimited) when the field is absent (nil).
// IEEE format implies a maximum of 5 authors when no explicit limit is set.
func (cfg *Config) maxAuthors() int {
	if cfg.MaxAuthors == nil || *cfg.MaxAuthors == 0 {
		if cfg.ieeeFormat() {
			return 5
		}
		return 0
	}
	return *cfg.MaxAuthors
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(".el/config.json")
	if err != nil {
		return nil, fmt.Errorf("not initialized. Run 'el init' first")
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("corrupt .el/config.json: %w", err)
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(".el/config.json", data, 0644)
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
	rootCmd.AddCommand(configCmd)
}
