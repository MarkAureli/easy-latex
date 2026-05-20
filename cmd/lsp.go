package cmd

import (
	"os"

	"github.com/MarkAureli/easy-latex/internal/lsp"
	"github.com/MarkAureli/easy-latex/internal/spell"
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

		lspCfg := lsp.Config{
			Items:         lsp.BuildItems(),
			EnabledChecks: enabled,
		}

		// Spell wiring — only when language configured AND we can resolve
		// both the global config dir and a project root (for the auxDir
		// personal-dict scratch file).
		if cfg != nil && cfg.Spelling != nil {
			if globalDir, err := GlobalConfigDir(); err == nil {
				lspCfg.SpellLang = *cfg.Spelling
				lspCfg.SpellAuxDir = auxDir
				lspCfg.SpellPaths = spell.DefaultPaths(globalDir, auxDir, *cfg.Spelling)
			}
		}

		return lsp.Serve(lspCfg, os.Stdin, os.Stdout)
	},
}
