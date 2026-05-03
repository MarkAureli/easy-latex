package cmd

import (
	"fmt"
	"os"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
	"github.com/MarkAureli/easy-latex/internal/term"
	"github.com/MarkAureli/easy-latex/internal/texscan"
	"github.com/spf13/cobra"
)

var checkFix bool

var checkCmd = &cobra.Command{
	Use:               "check",
	Short:             "Run static pedantic checks (no compile)",
	SilenceUsage:      true,
	RunE:              runCheck,
	ValidArgsFunction: cobra.NoFileCompletions,
}

func init() {
	checkCmd.Flags().BoolVarP(&checkFix, "fix", "f", false, "Apply autofixes to source files where available")
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	enabled, err := effectiveEnabledChecks(cfg)
	if err != nil {
		return err
	}
	if len(enabled) == 0 {
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

	if checkFix {
		modified, err := pedantic.RunSourceFixes(enabled, texFiles)
		if err != nil {
			return err
		}
		if len(modified) == 0 {
			fmt.Println("No fixes applied.")
		} else {
			fmt.Printf("Applied fixes to %d file(s):\n", len(modified))
			for _, p := range modified {
				fmt.Printf("  %s%s%s\n", colors.Green, p, colors.Reset)
			}
		}
	}

	diags := pedantic.RunSourceChecks(enabled, texFiles)
	if len(diags) == 0 {
		fmt.Println("No issues found.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "%s%sPedantic:%s\n", colors.Bold, colors.Red, colors.Reset)
	for _, d := range diags {
		fmt.Fprintf(os.Stderr, "  %s%s%s\n", colors.Red, d.String(), colors.Reset)
	}
	return fmt.Errorf("pedantic checks failed (%d violations)", len(diags))
}
