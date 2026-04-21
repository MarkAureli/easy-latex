package cmd

import (
	"fmt"

	"github.com/MarkAureli/easy-latex/internal/bib"
	"github.com/spf13/cobra"
)

var bibentryCmd = &cobra.Command{
	Use:               "bibentry <ID>",
	Short:             "Add a bib cache entry from a DOI or arXiv ID",
	Args:              cobra.ExactArgs(1),
	RunE:              runBibentry,
	ValidArgsFunction: cobra.NoFileCompletions,
}

func runBibentry(cmd *cobra.Command, args []string) error {
	key, err := bib.AddEntryFromID(args[0], auxDir)
	if err != nil {
		if err == bib.ErrUnrecognizedID {
			fmt.Fprintf(cmd.ErrOrStderr(), "[bib] warning: %q is not a valid DOI or arXiv identifier\n", args[0])
			return nil
		}
		return err
	}
	fmt.Printf("Added %q to bib cache.\n", key)
	return nil
}
