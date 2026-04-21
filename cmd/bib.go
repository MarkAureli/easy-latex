package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/MarkAureli/easy-latex/internal/bib"
	"github.com/MarkAureli/easy-latex/internal/term"
	"github.com/spf13/cobra"
)

var bibCmd = &cobra.Command{
	Use:   "bib",
	Short: "Manage the bibliography cache",
}

var bibListCmd = &cobra.Command{
	Use:               "list",
	Short:             "List all entries in the bib cache",
	RunE:              runBibList,
	ValidArgsFunction: cobra.NoFileCompletions,
}

var bibAddCmd = &cobra.Command{
	Use:               "add <ID>",
	Short:             "Add an entry from a DOI or arXiv ID",
	Args:              cobra.ExactArgs(1),
	RunE:              runBibAdd,
	ValidArgsFunction: cobra.NoFileCompletions,
}

func init() {
	bibCmd.AddCommand(bibListCmd)
	bibCmd.AddCommand(bibAddCmd)
}

func runBibList(cmd *cobra.Command, args []string) error {
	entries := bib.LoadCacheEntries(auxDir)
	if len(entries) == 0 {
		fmt.Println("No entries in bib cache.")
		return nil
	}

	typeW, srcW := len("TYPE"), len("SOURCE")
	for _, e := range entries {
		typeW = max(typeW, len(e.Type))
		srcW = max(srcW, len(e.Source))
	}
	keyMax := max(15, term.Width()-typeW-srcW-4) // 4 = 2 padding per column gap
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	fmt.Fprintf(w, "KEY\tTYPE\tSOURCE\n")
	for _, e := range entries {
		key := truncate(e.Key, keyMax)
		fmt.Fprintf(w, "%s\t%s\t%s\n", key, e.Type, e.Source)
	}
	w.Flush()
	fmt.Printf("\n%d entries in bib cache.\n", len(entries))
	return nil
}

func runBibAdd(cmd *cobra.Command, args []string) error {
	log := newBibLogger()
	key, isNew, err := bib.AddEntryFromID(args[0], auxDir, log)
	if err != nil {
		if err == bib.ErrUnrecognizedID {
			fmt.Fprintf(cmd.ErrOrStderr(), "[bib] warning: %q is not a valid DOI or arXiv identifier\n", args[0])
			return nil
		}
		return err
	}
	if !isNew {
		fmt.Printf("%q already in bib cache.\n", key)
		return nil
	}
	// Show entry details from cache.
	entries := bib.LoadCacheEntries(auxDir)
	for _, e := range entries {
		if e.Key == key {
			fmt.Printf("Added %q to bib cache.\n", key)
			if e.Title != "" {
				fmt.Printf("  Title:  %s\n", e.Title)
			}
			if e.Author != "" {
				fmt.Printf("  Author: %s\n", truncateAuthor(e.Author))
			}
			fmt.Printf("  Source: %s\n", e.Source)
			return nil
		}
	}
	fmt.Printf("Added %q to bib cache.\n", key)
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func truncateAuthor(s string) string {
	parts := strings.SplitN(s, " and ", 2)
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + " et al."
}
