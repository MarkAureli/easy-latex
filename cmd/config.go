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
	flagAbbreviateJournals  bool
	flagBraceTitles         bool
	flagIEEEFormat          bool
	flagMaxAuthors          int
	flagAbbreviateFirstName bool
	flagUrlFromDOI          bool
)

func init() {
	configCmd.Flags().BoolVar(&flagAbbreviateJournals, "abbreviate-journals", true,
		"Abbreviate journal names according to ISO 4 (default true)")
	configCmd.Flags().BoolVar(&flagBraceTitles, "brace-titles", false,
		"Enclose title values in an extra pair of curly braces (default false)")
	configCmd.Flags().BoolVar(&flagIEEEFormat, "ieee-format", false,
		"Enable IEEE bib formatting: forces brace-titles and converts arXiv @misc to @unpublished (default false)")
	configCmd.Flags().IntVar(&flagMaxAuthors, "max-authors", 0,
		"Maximum number of authors to store (0 = unlimited; >=1 truncates to N and appends 'and others')")
	configCmd.Flags().BoolVar(&flagAbbreviateFirstName, "abbreviate-first-name", true,
		"Abbreviate first (and middle) names to initials (default true; false keeps first name in full)")
	configCmd.Flags().BoolVar(&flagUrlFromDOI, "url-from-doi", false,
		"Replace url field with https://doi.org/<doi> for entries with a non-empty doi (default false)")
}

func runConfig(cmd *cobra.Command, args []string) error {
	abbrevChanged := cmd.Flags().Changed("abbreviate-journals")
	braceChanged := cmd.Flags().Changed("brace-titles")
	ieeeChanged := cmd.Flags().Changed("ieee-format")
	maxAuthorsChanged := cmd.Flags().Changed("max-authors")
	abbrevFirstChanged := cmd.Flags().Changed("abbreviate-first-name")
	urlFromDOIChanged := cmd.Flags().Changed("url-from-doi")

	if !abbrevChanged && !braceChanged && !ieeeChanged && !maxAuthorsChanged && !abbrevFirstChanged && !urlFromDOIChanged {
		return fmt.Errorf("no options specified. Use --abbreviate-journals=<true|false>, --brace-titles=<true|false>, --ieee-format=<true|false>, --max-authors=<N>, --abbreviate-first-name=<true|false>, or --url-from-doi=<true|false>")
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

	if ieeeChanged {
		v := flagIEEEFormat
		cfg.IEEEFormat = &v
		if flagIEEEFormat {
			fmt.Println("IEEE bib formatting enabled")
			fmt.Println("Max authors set to 5 (IEEE default; override with --max-authors)")
		} else {
			fmt.Println("IEEE bib formatting disabled")
		}
	}

	if maxAuthorsChanged {
		if flagMaxAuthors < 0 {
			return fmt.Errorf("--max-authors must be 0 (unlimited) or a positive integer")
		}
		v := flagMaxAuthors
		cfg.MaxAuthors = &v
		if flagMaxAuthors == 0 {
			fmt.Println("Max authors set to unlimited")
		} else {
			fmt.Printf("Max authors set to %d\n", flagMaxAuthors)
		}
	}

	if abbrevFirstChanged {
		v := flagAbbreviateFirstName
		cfg.AbbreviateFirstName = &v
		if flagAbbreviateFirstName {
			fmt.Println("First name abbreviation enabled")
		} else {
			fmt.Println("First name abbreviation disabled (first name kept in full)")
		}
	}

	if urlFromDOIChanged {
		v := flagUrlFromDOI
		cfg.UrlFromDOI = &v
		if flagUrlFromDOI {
			fmt.Println("URL-from-DOI enabled (url field replaced with https://doi.org/<doi> when doi is present)")
		} else {
			fmt.Println("URL-from-DOI disabled (url field only set from doi when absent)")
		}
	}

	return saveConfig(cfg)
}
