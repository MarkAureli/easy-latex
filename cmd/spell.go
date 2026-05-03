package cmd

import (
	"fmt"
	"os"
	"slices"

	"github.com/MarkAureli/easy-latex/internal/spell"
	"github.com/spf13/cobra"
)

var (
	spellGlobal bool
	spellIgnore bool
	spellCommon bool
)

var spellCmd = &cobra.Command{
	Use:               "spell",
	Short:             "Manage spell-check dictionaries and ignore lists",
	RunE:              func(_ *cobra.Command, _ []string) error { return fmt.Errorf("usage: el spell add|remove|list [...]") },
	ValidArgsFunction: cobra.NoFileCompletions,
}

var spellAddCmd = &cobra.Command{
	Use:               "add <token>...",
	Short:             "Add words to a dict, or macros to the ignore list",
	Args:              cobra.MinimumNArgs(1),
	RunE:              runSpellAdd,
	ValidArgsFunction: cobra.NoFileCompletions,
}

var spellRemoveCmd = &cobra.Command{
	Use:               "remove <token>...",
	Short:             "Remove words from a dict, or un-ignore macros",
	Args:              cobra.MinimumNArgs(1),
	RunE:              runSpellRemove,
	ValidArgsFunction: spellRemoveCompletion,
}

var spellListCmd = &cobra.Command{
	Use:               "list",
	Short:             "Print contents of the resolved dict or ignore file",
	Args:              cobra.NoArgs,
	RunE:              runSpellList,
	ValidArgsFunction: cobra.NoFileCompletions,
}

func init() {
	for _, c := range []*cobra.Command{spellAddCmd, spellRemoveCmd, spellListCmd} {
		c.Flags().BoolVar(&spellGlobal, "global", false, "Operate on the global file (instead of project-local)")
		c.Flags().BoolVar(&spellIgnore, "ignore", false, "Operate on the macro-arg ignore file")
		c.Flags().BoolVar(&spellCommon, "common", false, "Operate on the language-agnostic common dict")
	}
	spellCmd.AddCommand(spellAddCmd)
	spellCmd.AddCommand(spellRemoveCmd)
	spellCmd.AddCommand(spellListCmd)
	rootCmd.AddCommand(spellCmd)
}

// resolveSpellTarget validates flags and returns the target file path.
func resolveSpellTarget(cmd *cobra.Command) (string, error) {
	isGlobal, _ := cmd.Flags().GetBool("global")
	isIgnore, _ := cmd.Flags().GetBool("ignore")
	isCommon, _ := cmd.Flags().GetBool("common")
	if isIgnore && isCommon {
		return "", fmt.Errorf("--ignore and --common are mutually exclusive")
	}

	var auxDirAbs string
	if !isGlobal {
		root, err := findProjectRoot()
		if err != nil {
			return "", err
		}
		if err := os.Chdir(root); err != nil {
			return "", err
		}
		auxDirAbs = auxDir
	}

	globalDir, err := GlobalConfigDir()
	if err != nil {
		return "", err
	}

	lang := ""
	if !isIgnore && !isCommon {
		cfg, err := loadConfig()
		if err != nil {
			return "", err
		}
		lang = cfg.spelling()
	}

	return spell.ResolveTarget(globalDir, auxDirAbs, lang, isGlobal, isIgnore, isCommon)
}

func runSpellAdd(cmd *cobra.Command, args []string) error {
	path, err := resolveSpellTarget(cmd)
	if err != nil {
		return err
	}
	isIgnore, _ := cmd.Flags().GetBool("ignore")
	added, err := spell.AddTokens(path, args, isIgnore)
	if err != nil {
		return err
	}
	fmt.Printf("%s: added %d (of %d)\n", path, added, len(args))
	return nil
}

func runSpellRemove(cmd *cobra.Command, args []string) error {
	path, err := resolveSpellTarget(cmd)
	if err != nil {
		return err
	}
	isIgnore, _ := cmd.Flags().GetBool("ignore")
	removed, negated, err := spell.RemoveTokens(path, args, isIgnore)
	if err != nil {
		return err
	}
	if isIgnore {
		fmt.Printf("%s: removed %d, negated %d default(s)\n", path, removed, negated)
	} else {
		fmt.Printf("%s: removed %d (of %d)\n", path, removed, len(args))
	}
	return nil
}

func runSpellList(cmd *cobra.Command, _ []string) error {
	path, err := resolveSpellTarget(cmd)
	if err != nil {
		return err
	}
	tokens, err := spell.ListTokens(path)
	if err != nil {
		return err
	}
	for _, t := range tokens {
		fmt.Println(t)
	}
	return nil
}

func spellRemoveCompletion(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	path, err := resolveSpellTarget(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	isIgnore, _ := cmd.Flags().GetBool("ignore")
	cands := spell.CompletionCandidates(path, isIgnore)
	out := cands[:0]
	for _, c := range cands {
		if !slices.Contains(args, c) {
			out = append(out, c)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}
