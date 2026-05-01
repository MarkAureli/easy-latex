package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
	"github.com/spf13/cobra"
)

// configField describes a single configurable setting.
type configField struct {
	key     string
	isBool  bool
	setVal  func(*Config, string) error // parse value string and set field
	unset   func(*Config)               // bool: set to false; int: clear to nil
	isSet   func(*Config) bool          // true when explicitly set (non-nil)
	display func(*Config) string        // effective value for display
}

// bibConfigFields lists configurable bib options (statically typed).
var bibConfigFields = []configField{
	{
		key: "abbreviate-journals", isBool: true,
		setVal:  bibBoolSetter(func(c *Config, v bool) { c.Bib.AbbreviateJournals = &v }),
		unset:   func(c *Config) { v := false; c.Bib.AbbreviateJournals = &v },
		isSet:   func(c *Config) bool { return c.Bib.AbbreviateJournals != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.abbreviateJournals()) },
	},
	{
		key: "abbreviate-first-name", isBool: true,
		setVal:  bibBoolSetter(func(c *Config, v bool) { c.Bib.AbbreviateFirstName = &v }),
		unset:   func(c *Config) { v := false; c.Bib.AbbreviateFirstName = &v },
		isSet:   func(c *Config) bool { return c.Bib.AbbreviateFirstName != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.abbreviateFirstName()) },
	},
	{
		key: "brace-titles", isBool: true,
		setVal:  bibBoolSetter(func(c *Config, v bool) { c.Bib.BraceTitles = &v }),
		unset:   func(c *Config) { v := false; c.Bib.BraceTitles = &v },
		isSet:   func(c *Config) bool { return c.Bib.BraceTitles != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.braceTitles()) },
	},
	{
		key: "arxiv-as-unpublished", isBool: true,
		setVal:  bibBoolSetter(func(c *Config, v bool) { c.Bib.ArxivAsUnpublished = &v }),
		unset:   func(c *Config) { v := false; c.Bib.ArxivAsUnpublished = &v },
		isSet:   func(c *Config) bool { return c.Bib.ArxivAsUnpublished != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.arxivAsUnpublished()) },
	},
	{
		key: "max-authors", isBool: false,
		setVal: func(c *Config, val string) error {
			n, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("invalid value %q for max-authors: must be an integer", val)
			}
			if n < 0 {
				return fmt.Errorf("max-authors must be 0 (unlimited) or a positive integer")
			}
			c.Bib.MaxAuthors = &n
			return nil
		},
		unset: func(c *Config) { c.Bib.MaxAuthors = nil },
		isSet: func(c *Config) bool { return c.Bib.MaxAuthors != nil },
		display: func(c *Config) string {
			v := c.maxAuthors()
			if v == 0 {
				return "0 (unlimited)"
			}
			return strconv.Itoa(v)
		},
	},
	{
		key: "url-from-doi", isBool: true,
		setVal:  bibBoolSetter(func(c *Config, v bool) { c.Bib.UrlFromDOI = &v }),
		unset:   func(c *Config) { v := false; c.Bib.UrlFromDOI = &v },
		isSet:   func(c *Config) bool { return c.Bib.UrlFromDOI != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.urlFromDOI()) },
	},
	{
		key: "retry-timeout", isBool: true,
		setVal:  bibBoolSetter(func(c *Config, v bool) { c.Bib.RetryTimeout = &v }),
		unset:   func(c *Config) { v := false; c.Bib.RetryTimeout = &v },
		isSet:   func(c *Config) bool { return c.Bib.RetryTimeout != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.retryTimeout()) },
	},
}

// pedanticConfigFields builds a configField per registered pedantic check.
// Each check is a bool toggle stored in cfg.Pedantic.Checks[name].
func pedanticConfigFields() []configField {
	names := pedantic.AllNames()
	fields := make([]configField, 0, len(names))
	for _, name := range names {
		n := name
		fields = append(fields, configField{
			key:    n,
			isBool: true,
			setVal: func(c *Config, val string) error {
				v, err := parseBool(val)
				if err != nil {
					return err
				}
				if c.Pedantic.Checks == nil {
					c.Pedantic.Checks = map[string]*bool{}
				}
				c.Pedantic.Checks[n] = &v
				return nil
			},
			unset: func(c *Config) {
				v := false
				if c.Pedantic.Checks == nil {
					c.Pedantic.Checks = map[string]*bool{}
				}
				c.Pedantic.Checks[n] = &v
			},
			isSet: func(c *Config) bool {
				v, ok := c.Pedantic.Checks[n]
				return ok && v != nil
			},
			display: func(c *Config) string {
				return strconv.FormatBool(c.Pedantic.Enabled(n))
			},
		})
	}
	return fields
}

// allConfigFields returns bib + pedantic fields. Built fresh each call so the
// pedantic registry can be populated by package init() before use.
func allConfigFields() []configField {
	out := make([]configField, 0, len(bibConfigFields)+8)
	out = append(out, bibConfigFields...)
	out = append(out, pedanticConfigFields()...)
	return out
}

// configFields is the public list, populated lazily on first access.
var configFields = allConfigFields()

// parseBool accepts "", "true", "false". Empty string treated as true.
func parseBool(val string) (bool, error) {
	switch val {
	case "", "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %q (use true or false)", val)
	}
}

// bibBoolSetter returns a setVal function for a bib boolean option.
func bibBoolSetter(set func(*Config, bool)) func(*Config, string) error {
	return func(c *Config, val string) error {
		v, err := parseBool(val)
		if err != nil {
			return err
		}
		set(c, v)
		return nil
	}
}

func findField(key string) *configField {
	for i := range configFields {
		if configFields[i].key == key {
			return &configFields[i]
		}
	}
	return nil
}

func validKeys() string {
	keys := make([]string, len(configFields))
	for i, f := range configFields {
		keys[i] = f.key
	}
	return strings.Join(keys, ", ")
}

// ── Commands ─────────────────────────────────────────────────────────────────

var configCmd = &cobra.Command{
	Use:               "config",
	Short:             "Display or modify easy-latex configuration",
	RunE:              runConfigCmd,
	ValidArgsFunction: cobra.NoFileCompletions,
}

var configListCmd = &cobra.Command{
	Use:               "list",
	Short:             "Display the effective configuration",
	Args:              cobra.NoArgs,
	RunE:              runConfigList,
	ValidArgsFunction: cobra.NoFileCompletions,
}

var configSetCmd = &cobra.Command{
	Use:               "set <key> [value]",
	Short:             "Set a configuration value",
	Args:              cobra.RangeArgs(1, 2),
	RunE:              runConfigSet,
	ValidArgsFunction: configKeyCompletion,
}

var configUnsetCmd = &cobra.Command{
	Use:               "unset <key>",
	Short:             "Unset a configuration value",
	Args:              cobra.ExactArgs(1),
	RunE:              runConfigUnset,
	ValidArgsFunction: configKeyCompletion,
}

func init() {
	configListCmd.Flags().Bool("global", false,
		"Show only global config (~/.elconfig.json)")
	configSetCmd.Flags().Bool("global", false,
		"Modify the global config (~/.elconfig.json) instead of the local project config")
	configUnsetCmd.Flags().Bool("global", false,
		"Modify the global config (~/.elconfig.json) instead of the local project config")
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configUnsetCmd)
}

func configKeyCompletion(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	keys := make([]string, 0, len(configFields)+1)
	for _, f := range configFields {
		keys = append(keys, f.key)
	}
	keys = append(keys, pedanticAliasKey)
	return keys, cobra.ShellCompDirectiveNoFileComp
}

// pedanticAliasKey is a convenience alias that toggles every registered
// pedantic check at once. It is not a configField — it has no display entry
// and is not stored in the config file under its own name.
const pedanticAliasKey = "pedantic"

// ── Display ──────────────────────────────────────────────────────────────────

func runConfigCmd(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("usage: el config list [--global] | el config set <key> [value] | el config unset <key>")
}

func runConfigList(cmd *cobra.Command, args []string) error {
	global, err := loadGlobalConfig()
	if err != nil {
		return err
	}

	globalOnly, _ := cmd.Flags().GetBool("global")
	if globalOnly {
		displayConfig(global, &Config{}, global)
		return nil
	}

	// Outside a project, show global config only.
	local := &Config{}
	if root, err := findProjectRoot(); err == nil {
		if err := os.Chdir(root); err != nil {
			return err
		}
		if l, err := loadLocalConfig(); err == nil {
			local = l
		}
	}
	merged := mergeConfig(local, global)
	displayConfig(merged, local, global)
	return nil
}

func displayConfig(merged, local, global *Config) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SETTING\tVALUE\tSOURCE")

	for _, f := range configFields {
		value := f.display(merged)
		source := configSource(f, local, global, merged)
		fmt.Fprintf(w, "%s\t%s\t%s\n", f.key, value, source)
	}
	w.Flush()
}

func configSource(f configField, local, global, _ *Config) string {
	if f.isSet(local) {
		return "(local)"
	}
	if f.isSet(global) {
		return "(global)"
	}
	return "(default)"
}

// ── Set / Unset ──────────────────────────────────────────────────────────────

// loadTargetConfig loads the appropriate config and returns a save function.
func loadTargetConfig(cmd *cobra.Command) (*Config, func(*Config) error, error) {
	global, _ := cmd.Flags().GetBool("global")
	if global {
		cfg, err := loadGlobalConfig()
		if err != nil {
			return nil, nil, err
		}
		return cfg, saveGlobalConfig, nil
	}
	root, err := findProjectRoot()
	if err != nil {
		return nil, nil, err
	}
	if err := os.Chdir(root); err != nil {
		return nil, nil, err
	}
	cfg, err := loadLocalConfig()
	if err != nil {
		return nil, nil, err
	}
	return cfg, saveLocalConfig, nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	val := ""
	if len(args) > 1 {
		val = args[1]
	}

	if key == pedanticAliasKey {
		cfg, save, err := loadTargetConfig(cmd)
		if err != nil {
			return err
		}
		for _, pf := range pedanticConfigFields() {
			if err := pf.setVal(cfg, val); err != nil {
				return err
			}
		}
		return save(cfg)
	}

	f := findField(key)
	if f == nil {
		return fmt.Errorf("unknown config key: %q\nValid keys: %s", key, validKeys())
	}
	if !f.isBool && val == "" {
		return fmt.Errorf("key %q requires a value", key)
	}

	cfg, save, err := loadTargetConfig(cmd)
	if err != nil {
		return err
	}
	if err := f.setVal(cfg, val); err != nil {
		return err
	}
	return save(cfg)
}

func runConfigUnset(cmd *cobra.Command, args []string) error {
	key := args[0]

	if key == pedanticAliasKey {
		cfg, save, err := loadTargetConfig(cmd)
		if err != nil {
			return err
		}
		for _, pf := range pedanticConfigFields() {
			pf.unset(cfg)
		}
		return save(cfg)
	}

	f := findField(key)
	if f == nil {
		return fmt.Errorf("unknown config key: %q\nValid keys: %s", key, validKeys())
	}

	cfg, save, err := loadTargetConfig(cmd)
	if err != nil {
		return err
	}
	f.unset(cfg)
	return save(cfg)
}
