package cmd

import (
	"fmt"
	"os"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
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
	rep, err := runSourceChecks(cfg, enabled, texscan.FindTexFiles(cfg.Main, "."), checkFix)
	if err != nil {
		return err
	}
	if len(rep.fixed) == 0 && len(rep.ped) == 0 && len(rep.spell) == 0 {
		fmt.Println("No issues found.")
		return nil
	}
	printReport(rep, nil, checkFix, false)
	return nil
}
