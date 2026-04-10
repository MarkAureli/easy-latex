package cmd

import (
	"os"

	"github.com/MarkAureli/easy-latex/internal/lsp"
	"github.com/spf13/cobra"
)

var lspCmd = &cobra.Command{
	Use:   "lsp",
	Short: "Start LSP server (cite-key completions over stdio)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		items := lsp.BuildItems(cfg.BibFiles)
		return lsp.Serve(items, os.Stdin, os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(lspCmd)
}
