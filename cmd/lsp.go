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
		items := lsp.BuildItems(auxDir)
		return lsp.Serve(items, os.Stdin, os.Stdout)
	},
}
