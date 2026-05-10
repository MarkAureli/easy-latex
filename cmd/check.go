package cmd

import (
	"fmt"
	"os"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
	"github.com/MarkAureli/easy-latex/internal/term"
	"github.com/MarkAureli/easy-latex/internal/texscan"
	"github.com/spf13/cobra"
)

var (
	checkFix      bool
	checkStrict   bool
	checkNoStrict bool
)

var checkCmd = &cobra.Command{
	Use:               "check",
	Short:             "Run static pedantic checks (no compile)",
	SilenceUsage:      true,
	RunE:              runCheck,
	ValidArgsFunction: cobra.NoFileCompletions,
}

func init() {
	checkCmd.Flags().BoolVarP(&checkFix, "fix", "f", false, "Apply autofixes to source files where available")
	checkCmd.Flags().BoolVar(&checkStrict, "strict", false, "Treat warnings as errors (overrides config)")
	checkCmd.Flags().BoolVar(&checkNoStrict, "no-strict", false, "Treat warnings as warnings (overrides config)")
	checkCmd.MarkFlagsMutuallyExclusive("strict", "no-strict")
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	enabled := cfg.Pedantic.EnabledNames()
	if len(enabled) == 0 && cfg.Spelling == nil {
		return fmt.Errorf("no pedantic checks enabled (configure with `el config set <check>`)")
	}
	if err := pedantic.ValidateCheckNames(enabled); err != nil {
		return err
	}

	if _, err := os.Stat(cfg.Main); err != nil {
		return fmt.Errorf("main file %q not found. Re-run 'el init'", cfg.Main)
	}
	texFiles := texscan.FindTexFiles(cfg.Main, ".")
	colors := term.Detect()

	var fixedPedDiags, pedDiags []pedantic.Diagnostic
	if checkFix {
		before := pedantic.RunSourceChecks(enabled, texFiles)
		if _, err := pedantic.RunSourceFixes(enabled, texFiles); err != nil {
			return err
		}
		pedDiags = pedantic.RunSourceChecks(enabled, texFiles)
		fixedPedDiags = diffDiagnostics(before, pedDiags)
	} else {
		pedDiags = pedantic.RunSourceChecks(enabled, texFiles)
	}
	spellDiags, err := runSpellCheck(cfg, texFiles)
	if err != nil {
		return err
	}
	sortDiagnostics(fixedPedDiags)
	sortDiagnostics(pedDiags)
	sortDiagnostics(spellDiags)

	if len(fixedPedDiags) == 0 && len(pedDiags) == 0 && len(spellDiags) == 0 {
		fmt.Println("No issues found.")
		return nil
	}

	remainingLabel := "Pedantics"
	if checkFix {
		remainingLabel = "Pedantics (remaining)"
	}
	first := true
	emit := func(fn func()) {
		if !first {
			fmt.Fprintln(os.Stderr)
		}
		fn()
		first = false
	}
	if len(fixedPedDiags) > 0 {
		emit(func() { printDiagSection(os.Stderr, "Pedantics (fixed)", fixedPedDiags, "", "", colors) })
	}
	if len(pedDiags) > 0 {
		emit(func() { printDiagSection(os.Stderr, remainingLabel, pedDiags, colors.Yellow, colors.Yellow, colors) })
	}
	if len(spellDiags) > 0 {
		emit(func() { printDiagSection(os.Stderr, "Misspellings", spellDiags, colors.Yellow, colors.Yellow, colors) })
	}
	printSummary(os.Stderr, len(pedDiags), len(spellDiags), 0, false, colors)
	if (len(pedDiags) > 0 || len(spellDiags) > 0) && resolveStrict(cfg, checkStrict, checkNoStrict) {
		return errStrict
	}
	return nil
}
