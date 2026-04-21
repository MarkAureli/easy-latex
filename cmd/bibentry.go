package cmd

import (
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
	return runBibAdd(cmd, args)
}
