package cmd

import (
	"fmt"

	"github.com/MarkAureli/easy-latex/internal/bib"
	"github.com/spf13/cobra"
)

var parsebibCmd = &cobra.Command{
	Use:   "parsebib",
	Short: "Allocate bib cache entries for all registered .bib files",
	RunE:  runParsebib,
}

func runParsebib(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	added, err := bib.AllocateCacheEntries(cfg.BibFiles, auxDir)
	if err != nil {
		return err
	}
	fmt.Printf("Allocated %d new bib cache entries.\n", added)
	return nil
}
