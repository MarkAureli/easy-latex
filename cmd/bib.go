package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/MarkAureli/easy-latex/internal/bib"
	"github.com/MarkAureli/easy-latex/internal/term"
	"github.com/MarkAureli/easy-latex/internal/texscan"
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

var bibParseCmd = &cobra.Command{
	Use:               "parse",
	Short:             "Allocate bib cache entries for all registered .bib files",
	RunE:              runBibParse,
	ValidArgsFunction: cobra.NoFileCompletions,
}

var bibRemoveCmd = &cobra.Command{
	Use:               "remove <key>",
	Short:             "Remove an entry from the bib cache",
	Args:              cobra.ExactArgs(1),
	RunE:              runBibRemove,
	ValidArgsFunction: bibKeyCompletion,
}

func init() {
	bibListCmd.Flags().Bool("cited", false, "Show only entries referenced in .tex files")
	bibListCmd.Flags().Bool("uncited", false, "Show only entries not referenced in .tex files")
	bibCmd.AddCommand(bibListCmd)
	bibCmd.AddCommand(bibAddCmd)
	bibCmd.AddCommand(bibParseCmd)
	bibCmd.AddCommand(bibRemoveCmd)
}

func runBibList(cmd *cobra.Command, args []string) error {
	entries := bib.LoadCacheEntries(auxDir)
	if len(entries) == 0 {
		fmt.Println("No entries in bib cache.")
		return nil
	}

	citedOnly, _ := cmd.Flags().GetBool("cited")
	uncitedOnly, _ := cmd.Flags().GetBool("uncited")

	// Try to resolve cited keys from tex files.
	var citedSet map[string]bool
	if cfg, err := loadConfig(); err == nil && cfg.Main != "" {
		keys := texscan.FindCiteKeys(cfg.Main, ".")
		citedSet = make(map[string]bool, len(keys))
		for _, k := range keys {
			citedSet[k] = true
		}
	}

	// Split into referenced/unreferenced when cite info available.
	var referenced, unreferenced []bib.CacheEntryInfo
	if citedSet != nil {
		for _, e := range entries {
			if citedSet[e.Key] {
				referenced = append(referenced, e)
			} else {
				unreferenced = append(unreferenced, e)
			}
		}
	}

	typeW, srcW := len("TYPE"), len("SOURCE")
	for _, e := range entries {
		typeW = max(typeW, len(e.Type))
		srcW = max(srcW, len(e.Source))
	}
	keyMax := max(15, term.Width()-typeW-srcW-4)
	out := cmd.OutOrStdout()
	c := term.Detect()

	colorSource := func(source string) string {
		switch source {
		case "crossref", "arxiv":
			return c.Green + source + c.Reset
		case "invalid-id":
			return c.Red + source + c.Reset
		case "timeout":
			return c.Yellow + source + c.Reset
		default: // "no-id" and others
			return c.Dim + source + c.Reset
		}
	}

	printSection := func(rows []bib.CacheEntryInfo) {
		w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
		fmt.Fprintf(w, "KEY\tTYPE\tSOURCE\n")
		for _, e := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\n", truncate(e.Key, keyMax), e.Type, colorSource(e.Source))
		}
		w.Flush()
	}

	switch {
	case citedSet == nil:
		// No config/tex — flat list, no grouping.
		printSection(entries)
	case citedOnly:
		printSection(referenced)
	case uncitedOnly:
		printSection(unreferenced)
	default:
		if len(referenced) > 0 {
			fmt.Fprintf(out, "── Referenced (%d) ──\n", len(referenced))
			printSection(referenced)
		}
		if len(unreferenced) > 0 {
			if len(referenced) > 0 {
				fmt.Fprintln(out)
			}
			fmt.Fprintf(out, "── Unreferenced (%d) ──\n", len(unreferenced))
			printSection(unreferenced)
		}
	}

	fmt.Fprintf(out, "\n%d entries in bib cache.\n", len(entries))
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

func runBibRemove(cmd *cobra.Command, args []string) error {
	key := args[0]
	removed, err := bib.RemoveEntryFromCache(key, auxDir)
	if err != nil {
		return err
	}
	if !removed {
		fmt.Fprintf(cmd.ErrOrStderr(), "[bib] warning: %q not found in bib cache\n", key)
		return nil
	}
	fmt.Printf("Removed %q from bib cache.\n", key)
	return nil
}

func bibKeyCompletion(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return bib.LoadCacheKeys(auxDir), cobra.ShellCompDirectiveNoFileComp
}

func runBibParse(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	log := newBibLogger()
	added, renames, err := bib.AllocateCacheEntries(cfg.BibFiles, auxDir, cfg.retryTimeout(), log)
	if err != nil {
		return err
	}
	for old, new := range renames {
		log.Info("", fmt.Sprintf("key renamed: %s -> %s", old, new))
	}
	bib.SaveRenames(auxDir, renames)
	if ef := entriesBibFile(cfg.BibFiles); ef != "" {
		bib.UpdateBibHash(ef, auxDir)
	}
	fmt.Printf("Allocated %d new bib cache entries.\n", added)
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
