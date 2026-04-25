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
	key      string
	isBool   bool
	setVal   func(*Config, string) error // parse value string and set field
	unset    func(*Config)               // bool: set to false; int/slice: clear to nil
	unsetVal   func(*Config, string) error // optional: remove specific value (slice fields)
	allowEmpty bool                        // allow set with no value
	isSet      func(*Config) bool          // true when pointer is non-nil
	display    func(*Config) string        // effective value for display
}

var configFields = []configField{
	{
		key: "abbreviate-journals", isBool: true,
		setVal:  boolSetter(func(c *Config, v bool) { c.AbbreviateJournals = &v }),
		unset:   func(c *Config) { v := false; c.AbbreviateJournals = &v },
		isSet:   func(c *Config) bool { return c.AbbreviateJournals != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.abbreviateJournals()) },
	},
	{
		key: "abbreviate-first-name", isBool: true,
		setVal:  boolSetter(func(c *Config, v bool) { c.AbbreviateFirstName = &v }),
		unset:   func(c *Config) { v := false; c.AbbreviateFirstName = &v },
		isSet:   func(c *Config) bool { return c.AbbreviateFirstName != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.abbreviateFirstName()) },
	},
	{
		key: "brace-titles", isBool: true,
		setVal:  boolSetter(func(c *Config, v bool) { c.BraceTitles = &v }),
		unset:   func(c *Config) { v := false; c.BraceTitles = &v },
		isSet:   func(c *Config) bool { return c.BraceTitles != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.braceTitles()) },
	},
	{
		key: "ieee-format", isBool: true,
		setVal:  boolSetter(func(c *Config, v bool) { c.IEEEFormat = &v }),
		unset:   func(c *Config) { v := false; c.IEEEFormat = &v },
		isSet:   func(c *Config) bool { return c.IEEEFormat != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.ieeeFormat()) },
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
			c.MaxAuthors = &n
			return nil
		},
		unset: func(c *Config) { c.MaxAuthors = nil },
		isSet: func(c *Config) bool { return c.MaxAuthors != nil },
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
		setVal:  boolSetter(func(c *Config, v bool) { c.UrlFromDOI = &v }),
		unset:   func(c *Config) { v := false; c.UrlFromDOI = &v },
		isSet:   func(c *Config) bool { return c.UrlFromDOI != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.urlFromDOI()) },
	},
	{
		key: "retry-timeout", isBool: true,
		setVal:  boolSetter(func(c *Config, v bool) { c.RetryTimeout = &v }),
		unset:   func(c *Config) { v := false; c.RetryTimeout = &v },
		isSet:   func(c *Config) bool { return c.RetryTimeout != nil },
		display: func(c *Config) string { return strconv.FormatBool(c.retryTimeout()) },
	},
	{
		key: "pedantic", isBool: false, allowEmpty: true,
		setVal: func(c *Config, val string) error {
			var names []string
			if val == "" {
				names = pedantic.AllNames()
			} else {
				names = splitCheckNames(val)
				if err := pedantic.ValidateCheckNames(names); err != nil {
					return err
				}
			}
			// Append, dedup
			seen := map[string]bool{}
			for _, n := range c.Pedantic {
				seen[n] = true
			}
			for _, n := range names {
				if !seen[n] {
					c.Pedantic = append(c.Pedantic, n)
					seen[n] = true
				}
			}
			return nil
		},
		unset: func(c *Config) { c.Pedantic = nil },
		unsetVal: func(c *Config, val string) error {
			remove := map[string]bool{}
			for _, n := range splitCheckNames(val) {
				remove[n] = true
			}
			var kept []string
			for _, n := range c.Pedantic {
				if !remove[n] {
					kept = append(kept, n)
				}
			}
			c.Pedantic = kept
			return nil
		},
		isSet: func(c *Config) bool { return len(c.Pedantic) > 0 },
		display: func(c *Config) string {
			if len(c.Pedantic) == 0 {
				return "(none)"
			}
			return strings.Join(c.Pedantic, ", ")
		},
	},
}

// boolSetter returns a setVal function for a boolean config field.
func boolSetter(set func(*Config, bool)) func(*Config, string) error {
	return func(c *Config, val string) error {
		switch val {
		case "", "true":
			set(c, true)
		case "false":
			set(c, false)
		default:
			return fmt.Errorf("invalid boolean value: %q (use true or false)", val)
		}
		return nil
	}
}

func splitCheckNames(val string) []string {
	var names []string
	for s := range strings.SplitSeq(val, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			names = append(names, s)
		}
	}
	return names
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
	Use:               "unset <key> [value]",
	Short:             "Unset a configuration value",
	Args:              cobra.RangeArgs(1, 2),
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
	keys := make([]string, len(configFields))
	for i, f := range configFields {
		keys[i] = f.key
	}
	return keys, cobra.ShellCompDirectiveNoFileComp
}

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

func configSource(f configField, local, global, merged *Config) string {
	if f.isSet(local) {
		return "(local)"
	}
	if f.isSet(global) {
		return "(global)"
	}
	if f.key == "max-authors" && merged.ieeeFormat() {
		return "(ieee default)"
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
	f := findField(key)
	if f == nil {
		return fmt.Errorf("unknown config key: %q\nValid keys: %s", key, validKeys())
	}

	val := ""
	if len(args) > 1 {
		val = args[1]
	}
	if !f.isBool && !f.allowEmpty && val == "" {
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
	f := findField(key)
	if f == nil {
		return fmt.Errorf("unknown config key: %q\nValid keys: %s", key, validKeys())
	}

	cfg, save, err := loadTargetConfig(cmd)
	if err != nil {
		return err
	}
	if len(args) > 1 && f.unsetVal != nil {
		if err := f.unsetVal(cfg, args[1]); err != nil {
			return err
		}
	} else {
		f.unset(cfg)
	}
	return save(cfg)
}
