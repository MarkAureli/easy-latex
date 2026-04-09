package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Modify easy-latex configuration",
	RunE:  runConfig,
}

var (
	flagAbbreviateJournals bool
	flagBraceTitles        bool
)

func init() {
	configCmd.Flags().BoolVar(&flagAbbreviateJournals, "abbreviate-journals", true,
		"Abbreviate journal names according to ISO 4 (default true)")
	configCmd.Flags().BoolVar(&flagBraceTitles, "brace-titles", false,
		"Enclose title values in an extra pair of curly braces (default false)")
}

func runConfig(cmd *cobra.Command, args []string) error {
	abbrevChanged := cmd.Flags().Changed("abbreviate-journals")
	braceChanged := cmd.Flags().Changed("brace-titles")

	if !abbrevChanged && !braceChanged {
		return fmt.Errorf("no options specified. Use --abbreviate-journals=<true|false> or --brace-titles=<true|false>")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if abbrevChanged {
		v := flagAbbreviateJournals
		cfg.AbbreviateJournals = &v
		if flagAbbreviateJournals {
			fmt.Println("Journal abbreviation enabled (ISO 4)")
		} else {
			fmt.Println("Journal abbreviation disabled")
		}
	}

	if braceChanged {
		v := flagBraceTitles
		cfg.BraceTitles = &v
		if flagBraceTitles {
			fmt.Println("Title double-bracing enabled")
		} else {
			fmt.Println("Title double-bracing disabled")
		}
	}

	return saveConfig(cfg)
}
