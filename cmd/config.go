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

var flagAbbreviateJournals bool

func init() {
	configCmd.Flags().BoolVar(&flagAbbreviateJournals, "abbreviate-journals", true,
		"Abbreviate journal names according to ISO 4 (default true)")
}

func runConfig(cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Changed("abbreviate-journals") {
		return fmt.Errorf("no options specified. Use --abbreviate-journals=<true|false>")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	v := flagAbbreviateJournals
	cfg.AbbreviateJournals = &v

	if err := saveConfig(cfg); err != nil {
		return err
	}

	if flagAbbreviateJournals {
		fmt.Println("Journal abbreviation enabled (ISO 4)")
	} else {
		fmt.Println("Journal abbreviation disabled")
	}
	return nil
}
