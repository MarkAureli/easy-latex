package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/bib"
	"github.com/MarkAureli/easy-latex/internal/pedantic"
	"github.com/MarkAureli/easy-latex/internal/spell"
	"github.com/MarkAureli/easy-latex/internal/term"
	"github.com/MarkAureli/easy-latex/internal/texscan"
	"github.com/spf13/cobra"
)

const auxDir = ".el"

// errStrict signals strict-mode failure. Execute() exits 1 without printing it
// — the per-section yellow output already conveyed the cause.
var errStrict = errors.New("strict mode: warnings present")

// Config is the structure stored in .el/config.json
type Config struct {
	Main     string         `json:"main"`
	BibFiles []string       `json:"bib_files,omitempty"`
	Bib      BibConfig      `json:"bib,omitzero"`
	Pedantic PedanticConfig `json:"pedantic,omitzero"`
	// Spelling selects spell-check language. nil = off. Allowed: "en_GB", "en_US".
	Spelling *string `json:"spelling,omitempty"`
	// Strict treats pedantic, spelling, and compile warnings as errors when true.
	Strict *bool `json:"strict,omitempty"`
}

func (cfg *Config) strict() bool {
	return cfg.Strict != nil && *cfg.Strict
}

// spelling returns the active spell-check language ("en_GB", "en_US") or empty
// when off.
func (cfg *Config) spelling() string {
	if cfg.Spelling == nil {
		return ""
	}
	return *cfg.Spelling
}

// BibConfig groups bibliography processing options.
type BibConfig struct {
	AbbreviateJournals  *bool `json:"abbreviate_journals,omitempty"`
	BraceTitles         *bool `json:"brace_titles,omitempty"`
	MaxAuthors          *int  `json:"max_authors,omitempty"`
	AbbreviateFirstName *bool `json:"abbreviate_first_name,omitempty"`
	UrlFromDOI          *bool `json:"url_from_doi,omitempty"`
	RetryTimeout        *bool `json:"retry_timeout,omitempty"`
	ArxivAsUnpublished  *bool `json:"arxiv_as_unpublished,omitempty"`
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

// arxivAsUnpublished returns true when arXiv @misc entries should be written as
// @unpublished. Defaults to false when the field is absent (nil).
func (cfg *Config) arxivAsUnpublished() bool {
	return cfg.Bib.ArxivAsUnpublished != nil && *cfg.Bib.ArxivAsUnpublished
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
func (cfg *Config) maxAuthors() int {
	if cfg.Bib.MaxAuthors == nil || *cfg.Bib.MaxAuthors == 0 {
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

// globalConfigDir overrides the global config directory.
// Empty means use ${XDG_CONFIG_HOME:-~/.config}/easy-latex. Set in tests for isolation.
var globalConfigDir string

// GlobalConfigDir returns the directory holding the global config and ancillary
// files (spell dicts, ignore lists, …). Honors globalConfigDir override, then
// XDG_CONFIG_HOME, then ~/.config.
func GlobalConfigDir() (string, error) {
	if globalConfigDir != "" {
		return globalConfigDir, nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "easy-latex"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "easy-latex"), nil
}

func globalConfigPath() (string, error) {
	dir, err := GlobalConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
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
	merged.Bib.MaxAuthors = mergeInt(local.Bib.MaxAuthors, global.Bib.MaxAuthors)
	merged.Bib.AbbreviateFirstName = mergeBool(local.Bib.AbbreviateFirstName, global.Bib.AbbreviateFirstName)
	merged.Bib.UrlFromDOI = mergeBool(local.Bib.UrlFromDOI, global.Bib.UrlFromDOI)
	merged.Bib.RetryTimeout = mergeBool(local.Bib.RetryTimeout, global.Bib.RetryTimeout)
	merged.Bib.ArxivAsUnpublished = mergeBool(local.Bib.ArxivAsUnpublished, global.Bib.ArxivAsUnpublished)

	mergeStr := func(l, g *string) *string {
		if l != nil {
			return l
		}
		return g
	}
	merged.Spelling = mergeStr(local.Spelling, global.Spelling)
	merged.Strict = mergeBool(local.Strict, global.Strict)

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
		case "init", "help", "completion", "lsp":
			return nil
		}
		if isConfigCommand(cmd) || isSpellCommand(cmd) {
			return nil
		}
		// bib subcommands target the global cache; only `bib parse` needs a
		// project (it consumes the project's registered .bib files).
		if isBibCommand(cmd) && cmd.Name() != "parse" {
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

// isBibCommand returns true if cmd is bibCmd or a child of bibCmd.
func isBibCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c == bibCmd {
			return true
		}
	}
	return false
}

// isSpellCommand returns true if cmd is spellCmd or a child of spellCmd.
// Spell subcommands resolve the project root themselves only when needed
// (i.e. without --global), so we skip the global PreRunE check.
func isSpellCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c == spellCmd {
			return true
		}
	}
	return false
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if !errors.Is(err, errStrict) {
			fmt.Fprintln(os.Stderr, err)
		}
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

// runSpellCheck runs the spell-checker on the project source if cfg.Spelling
// is set. Returns spell findings reshaped as pedantic.Diagnostic so callers
// can merge them with pedantic check output.
func runSpellCheck(cfg *Config, texFiles []string) ([]pedantic.Diagnostic, error) {
	if cfg.Spelling == nil {
		return nil, nil
	}
	globalDir, err := GlobalConfigDir()
	if err != nil {
		return nil, err
	}
	files := make(map[string][]string, len(texFiles))
	for _, p := range texFiles {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		raw := strings.Split(string(data), "\n")
		stripped := make([]string, len(raw))
		for i, l := range raw {
			stripped[i] = texscan.StripComment(l)
		}
		files[p] = stripped
	}
	paths := spell.DefaultPaths(globalDir, auxDir, *cfg.Spelling)
	sd := spell.Run(files, *cfg.Spelling, auxDir, paths, os.Stderr)
	out := make([]pedantic.Diagnostic, len(sd))
	for i, d := range sd {
		out[i] = pedantic.Diagnostic{File: d.File, Line: d.Line, Message: d.Message}
	}
	return out, nil
}

// resolveStrict returns the effective strict flag. CLI overrides config.
func resolveStrict(cfg *Config, strictFlag, noStrictFlag bool) bool {
	if strictFlag {
		return true
	}
	if noStrictFlag {
		return false
	}
	return cfg.strict()
}

// printDiagSection prints a labelled section of diagnostics. headerColor and
// entryColor are ANSI sequences (use "" for default colour). Caller is
// responsible for blank-line separation between adjacent sections.
func printDiagSection(w *os.File, label string, diags []pedantic.Diagnostic, headerColor, entryColor string, colors term.Colors) {
	if len(diags) == 0 {
		return
	}
	fmt.Fprintf(w, "%s%s%s:%s\n", colors.Bold, headerColor, label, colors.Reset)
	for _, d := range diags {
		fmt.Fprintf(w, "  %s%s%s\n", entryColor, d.String(), colors.Reset)
	}
}

// diffDiagnostics returns the diagnostics in `before` that are not present in
// `after` (matched by File+Line+Message). Used to derive the "fixed" subset
// after running source autofixes.
func diffDiagnostics(before, after []pedantic.Diagnostic) []pedantic.Diagnostic {
	key := func(d pedantic.Diagnostic) string {
		return fmt.Sprintf("%s\x00%d\x00%s", d.File, d.Line, d.Message)
	}
	seen := make(map[string]int, len(after))
	for _, d := range after {
		seen[key(d)]++
	}
	var fixed []pedantic.Diagnostic
	for _, d := range before {
		k := key(d)
		if seen[k] > 0 {
			seen[k]--
			continue
		}
		fixed = append(fixed, d)
	}
	return fixed
}

// printSummary prints a one-line yellow summary if any count is non-zero.
// includeWarnings controls whether the compile warnings count appears (compile only).
func printSummary(w *os.File, ped, spell, warn int, includeWarnings bool, colors term.Colors) {
	if ped == 0 && spell == 0 && warn == 0 {
		return
	}
	plural := func(n int, sing, pl string) string {
		if n == 1 {
			return fmt.Sprintf("%d %s", n, sing)
		}
		return fmt.Sprintf("%d %s", n, pl)
	}
	fmt.Fprintln(w)
	parts := []string{
		plural(ped, "pedantic", "pedantics"),
		plural(spell, "misspelling", "misspellings"),
	}
	if includeWarnings {
		parts = append(parts, plural(warn, "warning", "warnings"))
	}
	summary := strings.Join(parts, ", ")
	fmt.Fprintf(w, "%s%s%s%s\n", colors.Bold, colors.Yellow, summary, colors.Reset)
}

// sortDiagnostics orders diagnostics by File then Line for stable output.
func sortDiagnostics(d []pedantic.Diagnostic) {
	sort.SliceStable(d, func(i, j int) bool {
		if d[i].File != d[j].File {
			return d[i].File < d[j].File
		}
		return d[i].Line < d[j].Line
	})
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
