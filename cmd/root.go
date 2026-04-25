package cmd

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"

	"github.com/MarkAureli/easy-latex/internal/bib"
	"github.com/spf13/cobra"
)

const auxDir = ".el"

// Config is the structure stored in .el/config.json
type Config struct {
	Main     string         `json:"main"`
	BibFiles []string       `json:"bib_files,omitempty"`
	Bib      BibConfig      `json:"bib,omitzero"`
	Pedantic PedanticConfig `json:"pedantic,omitzero"`
}

// BibConfig groups bibliography processing options.
type BibConfig struct {
	AbbreviateJournals  *bool `json:"abbreviate_journals,omitempty"`
	BraceTitles         *bool `json:"brace_titles,omitempty"`
	IEEEFormat          *bool `json:"ieee_format,omitempty"`
	MaxAuthors          *int  `json:"max_authors,omitempty"`
	AbbreviateFirstName *bool `json:"abbreviate_first_name,omitempty"`
	UrlFromDOI          *bool `json:"url_from_doi,omitempty"`
	RetryTimeout        *bool `json:"retry_timeout,omitempty"`
}

// PedanticConfig holds per-check enable/disable state. Map key = check name,
// value = pointer-to-bool (nil = inherit, *true = enabled, *false = disabled).
type PedanticConfig struct {
	Checks map[string]*bool `json:"checks,omitempty"`
}

// Enabled reports whether the named check is explicitly enabled.
func (p PedanticConfig) Enabled(name string) bool {
	if v, ok := p.Checks[name]; ok && v != nil {
		return *v
	}
	return false
}

// EnabledNames returns sorted names of checks set to true.
func (p PedanticConfig) EnabledNames() []string {
	var out []string
	for name, v := range p.Checks {
		if v != nil && *v {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// abbreviateJournals returns true when journal abbreviation is enabled.
// Defaults to true when the field is absent (nil).
func (cfg *Config) abbreviateJournals() bool {
	return cfg.Bib.AbbreviateJournals == nil || *cfg.Bib.AbbreviateJournals
}

// braceTitles returns true when title double-bracing is enabled.
// Defaults to false when the field is absent (nil).
func (cfg *Config) braceTitles() bool {
	return cfg.Bib.BraceTitles != nil && *cfg.Bib.BraceTitles
}

// ieeeFormat returns true when IEEE bib formatting is enabled.
// Defaults to false when the field is absent (nil).
func (cfg *Config) ieeeFormat() bool {
	return cfg.Bib.IEEEFormat != nil && *cfg.Bib.IEEEFormat
}

// abbreviateFirstName returns true when first (and middle) names should be
// abbreviated to initials. Defaults to true when the field is absent (nil).
func (cfg *Config) abbreviateFirstName() bool {
	return cfg.Bib.AbbreviateFirstName == nil || *cfg.Bib.AbbreviateFirstName
}

// urlFromDOI returns true when url fields should be replaced with https://doi.org/<doi>
// for entries with a non-empty doi field. Defaults to false when the field is absent (nil).
func (cfg *Config) urlFromDOI() bool {
	return cfg.Bib.UrlFromDOI != nil && *cfg.Bib.UrlFromDOI
}

// retryTimeout returns true when entries that previously failed validation due
// to a timeout should be automatically re-validated. Defaults to true.
func (cfg *Config) retryTimeout() bool {
	return cfg.Bib.RetryTimeout == nil || *cfg.Bib.RetryTimeout
}

// maxAuthors returns the maximum number of authors to store, or 0 for unlimited.
// Defaults to 0 (unlimited) when the field is absent (nil).
// IEEE format implies a maximum of 5 authors when no explicit limit is set.
func (cfg *Config) maxAuthors() int {
	if cfg.Bib.MaxAuthors == nil || *cfg.Bib.MaxAuthors == 0 {
		if cfg.ieeeFormat() {
			return 5
		}
		return 0
	}
	return *cfg.Bib.MaxAuthors
}

func loadLocalConfig() (*Config, error) {
	data, err := os.ReadFile(".el/config.json")
	if err != nil {
		return nil, fmt.Errorf("cannot read .el/config.json: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("corrupt .el/config.json: %w", err)
	}
	return &cfg, nil
}

func saveLocalConfig(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(".el/config.json", data, 0644)
}

// globalConfigDir overrides the home directory for the global config file.
// Empty means use os.UserHomeDir(). Set in tests for isolation.
var globalConfigDir string

func globalConfigPath() (string, error) {
	if globalConfigDir != "" {
		return filepath.Join(globalConfigDir, ".elconfig.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".elconfig.json"), nil
}

func loadGlobalConfig() (*Config, error) {
	path, err := globalConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("corrupt %s: %w", path, err)
	}
	return &cfg, nil
}

func saveGlobalConfig(cfg *Config) error {
	path, err := globalConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// mergeConfig returns a new Config where local pointer fields win over global.
// Main and BibFiles always come from local (project-specific).
func mergeConfig(local, global *Config) *Config {
	merged := *local
	mergeBool := func(l, g *bool) *bool {
		if l != nil {
			return l
		}
		return g
	}
	mergeInt := func(l, g *int) *int {
		if l != nil {
			return l
		}
		return g
	}
	merged.Bib.AbbreviateJournals = mergeBool(local.Bib.AbbreviateJournals, global.Bib.AbbreviateJournals)
	merged.Bib.BraceTitles = mergeBool(local.Bib.BraceTitles, global.Bib.BraceTitles)
	merged.Bib.IEEEFormat = mergeBool(local.Bib.IEEEFormat, global.Bib.IEEEFormat)
	merged.Bib.MaxAuthors = mergeInt(local.Bib.MaxAuthors, global.Bib.MaxAuthors)
	merged.Bib.AbbreviateFirstName = mergeBool(local.Bib.AbbreviateFirstName, global.Bib.AbbreviateFirstName)
	merged.Bib.UrlFromDOI = mergeBool(local.Bib.UrlFromDOI, global.Bib.UrlFromDOI)
	merged.Bib.RetryTimeout = mergeBool(local.Bib.RetryTimeout, global.Bib.RetryTimeout)

	// Pedantic: per-key pointer merge (local wins for keys it sets).
	if len(local.Pedantic.Checks) > 0 || len(global.Pedantic.Checks) > 0 {
		mergedChecks := map[string]*bool{}
		maps.Copy(mergedChecks, global.Pedantic.Checks)
		maps.Copy(mergedChecks, local.Pedantic.Checks)
		merged.Pedantic = PedanticConfig{Checks: mergedChecks}
	}
	return &merged
}

// loadConfig loads the local project config merged with the global config.
// Local settings take precedence over global settings.
func loadConfig() (*Config, error) {
	local, err := loadLocalConfig()
	if err != nil {
		return nil, err
	}
	global, err := loadGlobalConfig()
	if err != nil {
		return nil, err
	}
	return mergeConfig(local, global), nil
}

var rootCmd = &cobra.Command{
	Use:           "el",
	Short:         "easy-latex: simple LaTeX compilation",
	Version:       bib.Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip project check for commands that work outside a project.
		switch cmd.Name() {
		case "init", "help", "completion":
			return nil
		}
		if isConfigCommand(cmd) {
			return nil
		}
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		return os.Chdir(root)
	},
}

// findProjectRoot walks from the current directory upward looking for a .el
// directory. Returns the directory containing .el, or an error if none found.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, auxDir)); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("fatal: not an el project (or any of the parent directories): .el")
		}
		dir = parent
	}
}

// isConfigCommand returns true if cmd is configCmd or a child of configCmd.
func isConfigCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c == configCmd {
			return true
		}
	}
	return false
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(compileCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(bibCmd)
	rootCmd.AddCommand(lspCmd)
}

// entriesBibFile returns the path of the entries bib file (bibliography.bib)
// from bibFiles, or empty string if not present.
func entriesBibFile(bibFiles []string) string {
	for _, f := range bibFiles {
		if filepath.Base(f) == "bibliography.bib" {
			return f
		}
	}
	return ""
}
