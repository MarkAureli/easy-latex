# el lsp (`lsp.go`)

Calls `lsp.BuildItems(auxDir)` (reads `.el/bib.json`), then `lsp.Serve(items, os.Stdin, os.Stdout)`. No config load, no flags.

See `internal/lsp/AGENT.md` for protocol and implementation details.
