# el lsp (`lsp.go`)

Loads merged config (silently — server starts even outside a project, just without diagnostics). Builds cite-key items via `lsp.BuildItems()` (reads the global bib cache; no project state required). Calls `lsp.Serve(lsp.Config{...}, os.Stdin, os.Stdout)`.

`EnabledChecks = cfg.Pedantic.EnabledNames()` enables push diagnostics + code actions for the listed static checks. Empty disables linting cleanly.

`SpellLang / SpellAuxDir / SpellPaths` enable spell-check push diagnostics (no quickfix). Populated only when `cfg.Spelling != nil` and `GlobalConfigDir()` resolves; paths built via `spell.DefaultPaths(globalDir, auxDir, lang)`. Hunspell session is opened once at startup and reused across every `didChange`.

See `internal/lsp/AGENT.md` for protocol and implementation details.
