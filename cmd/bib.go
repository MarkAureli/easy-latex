package cmd

import (
	"fmt"
	"strings"

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
	Use:               "add <ID> [<ID>...]",
	Short:             "Add one or more entries from DOIs or arXiv IDs",
	Args:              cobra.MinimumNArgs(1),
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
	Use:               "remove <key> [<key>...]",
	Short:             "Remove one or more entries from the bib cache",
	Args:              cobra.MinimumNArgs(1),
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
	keyW := len("KEY")
	for _, e := range entries {
		k := truncate(e.Key, keyMax)
		keyW = max(keyW, len(k))
	}
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
		fmt.Fprintf(out, "%-*s  %-*s  %s\n", keyW, "KEY", typeW, "TYPE", "SOURCE")
		for _, e := range rows {
			fmt.Fprintf(out, "%-*s  %-*s  %s\n", keyW, truncate(e.Key, keyMax), typeW, e.Type, colorSource(e.Source))
		}
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

	// Batch-prefetch any arXiv ids in the argument list so multiple adds
	// share a single arXiv API call.
	var arxivIDs []string
	for _, a := range args {
		if id := bib.NormalizeArxivID(a); id != "" {
			arxivIDs = append(arxivIDs, id)
		}
	}
	bib.PrefetchArxivIDs(arxivIDs, log)

	var firstErr error
	for _, arg := range args {
		key, isNew, err := bib.AddEntryFromID(arg, auxDir, log)
		if err != nil {
			if err == bib.ErrUnrecognizedID {
				fmt.Fprintf(cmd.ErrOrStderr(), "[bib] warning: %q is not a valid DOI or arXiv identifier\n", arg)
				continue
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "[bib] error: %q: %v\n", arg, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if !isNew {
			fmt.Printf("%q already in bib cache.\n", key)
			continue
		}
		entry := findCacheEntry(auxDir, key)
		fmt.Printf("Added %q to bib cache.\n", key)
		if entry != nil {
			if entry.Title != "" {
				fmt.Printf("  Title:  %s\n", entry.Title)
			}
			if entry.Author != "" {
				fmt.Printf("  Author: %s\n", truncateAuthor(entry.Author))
			}
			fmt.Printf("  Source: %s\n", entry.Source)
		}
	}
	return firstErr
}

func findCacheEntry(auxDir, key string) *bib.CacheEntryInfo {
	for _, e := range bib.LoadCacheEntries(auxDir) {
		if e.Key == key {
			return &e
		}
	}
	return nil
}

func runBibRemove(cmd *cobra.Command, args []string) error {
	removed, notFound, err := bib.RemoveEntriesFromCache(args, auxDir)
	if err != nil {
		return err
	}
	for _, key := range removed {
		fmt.Printf("Removed %q from bib cache.\n", key)
	}
	for _, key := range notFound {
		fmt.Fprintf(cmd.ErrOrStderr(), "[bib] warning: %q not found in bib cache\n", key)
	}
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
