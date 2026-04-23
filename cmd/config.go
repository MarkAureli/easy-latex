package cmd

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:               "config",
	Short:             "Modify easy-latex configuration",
	RunE:              runConfig,
	ValidArgsFunction: cobra.NoFileCompletions,
}

var (
	flagAbbreviateJournals  bool
	flagBraceTitles         bool
	flagIEEEFormat          bool
	flagMaxAuthors          int
	flagAbbreviateFirstName bool
	flagUrlFromDOI          bool
	flagRetryTimeout        bool
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
	configCmd.Flags().BoolVar(&flagRetryTimeout, "retry-timeout", true,
		"Automatically retry validation for entries that previously timed out (default true)")
}

func runConfig(cmd *cobra.Command, args []string) error {
	abbrevChanged := cmd.Flags().Changed("abbreviate-journals")
	braceChanged := cmd.Flags().Changed("brace-titles")
	ieeeChanged := cmd.Flags().Changed("ieee-format")
	maxAuthorsChanged := cmd.Flags().Changed("max-authors")
	abbrevFirstChanged := cmd.Flags().Changed("abbreviate-first-name")
	urlFromDOIChanged := cmd.Flags().Changed("url-from-doi")
	retryTimeoutChanged := cmd.Flags().Changed("retry-timeout")

	if !abbrevChanged && !braceChanged && !ieeeChanged && !maxAuthorsChanged && !abbrevFirstChanged && !urlFromDOIChanged && !retryTimeoutChanged {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		displayConfig(cfg)
		return nil
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

	if retryTimeoutChanged {
		v := flagRetryTimeout
		cfg.RetryTimeout = &v
		if flagRetryTimeout {
			fmt.Println("Retry-timeout enabled (timed-out entries re-validated on next parse)")
		} else {
			fmt.Println("Retry-timeout disabled (timed-out entries kept as-is)")
		}
	}

	return saveConfig(cfg)
}

func displayConfig(cfg *Config) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SETTING\tVALUE\tSOURCE")

	row := func(name, value, source string) {
		fmt.Fprintf(w, "%s\t%s\t%s\n", name, value, source)
	}

	source := func(isNil bool) string {
		if isNil {
			return "(default)"
		}
		return "(set)"
	}

	row("abbreviate-journals", strconv.FormatBool(cfg.abbreviateJournals()), source(cfg.AbbreviateJournals == nil))
	row("abbreviate-first-name", strconv.FormatBool(cfg.abbreviateFirstName()), source(cfg.AbbreviateFirstName == nil))
	row("brace-titles", strconv.FormatBool(cfg.braceTitles()), source(cfg.BraceTitles == nil))
	row("ieee-format", strconv.FormatBool(cfg.ieeeFormat()), source(cfg.IEEEFormat == nil))

	// max-authors: special display for 0 (unlimited) and ieee default
	maxVal := cfg.maxAuthors()
	var maxStr string
	if maxVal == 0 {
		maxStr = "0 (unlimited)"
	} else {
		maxStr = strconv.Itoa(maxVal)
	}
	maxSource := source(cfg.MaxAuthors == nil)
	if cfg.MaxAuthors == nil && cfg.ieeeFormat() {
		maxSource = "(ieee default)"
	}
	row("max-authors", maxStr, maxSource)

	row("url-from-doi", strconv.FormatBool(cfg.urlFromDOI()), source(cfg.UrlFromDOI == nil))
	row("retry-timeout", strconv.FormatBool(cfg.retryTimeout()), source(cfg.RetryTimeout == nil))

	w.Flush()
}
