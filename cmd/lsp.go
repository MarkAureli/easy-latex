package cmd

import (
	"os"

	"github.com/MarkAureli/easy-latex/internal/lsp"
	"github.com/spf13/cobra"
)

var lspCmd = &cobra.Command{
	Use:               "lsp",
	Short:             "Start LSP server (cite-key completions over stdio)",
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := loadConfig()
		var enabled []string
		if cfg != nil {
			enabled = cfg.Pedantic.EnabledNames()
		}
		return lsp.Serve(lsp.Config{
			Items:         lsp.BuildItems(auxDir),
			EnabledChecks: enabled,
		}, os.Stdin, os.Stdout)
	},
}
