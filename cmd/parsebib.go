package cmd

import (
	"fmt"

	"github.com/MarkAureli/easy-latex/internal/bib"
	"github.com/spf13/cobra"
)

var parsebibCmd = &cobra.Command{
	Use:               "parsebib",
	Short:             "Allocate bib cache entries for all registered .bib files",
	RunE:              runParsebib,
	ValidArgsFunction: cobra.NoFileCompletions,
}

func runParsebib(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	log := newBibLogger()
	added, renames, err := bib.AllocateCacheEntries(cfg.BibFiles, auxDir, log)
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
