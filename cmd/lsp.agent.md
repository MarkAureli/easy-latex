# el lsp (`lsp.go`)

Loads merged config (silently — server starts even outside a project, just without diagnostics). Builds cite-key items via `lsp.BuildItems(auxDir)`. Calls `lsp.Serve(lsp.Config{Items, EnabledChecks}, os.Stdin, os.Stdout)`.

`EnabledChecks = cfg.Pedantic.EnabledNames()` enables push diagnostics + code actions for the listed static checks. Empty disables linting cleanly.

See `internal/lsp/AGENT.md` for protocol and implementation details.
