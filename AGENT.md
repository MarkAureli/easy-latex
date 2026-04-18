# easy-latex

CLI tool (`el`) for compiling LaTeX docs. Go project, module `github.com/MarkAureli/easy-latex`.

## Key files

| Path | Role |
|---|---|
| `.el/` | Working directory: config, all pdflatex/bibtex/biber intermediates, bib cache |
| `.el/config.json` | Config: main tex file, aux dir, bib paths, processing options |
| `.el/bib.json` | Per-entry validation source cache |
| `cmd/` | Cobra commands (`bibentry`, `compile`, `config`, `init`, `lsp`, `parsebib`) |
| `internal/bib/` | Bib parsing, key gen, formatting, validation |
| `internal/texscan/` | Tex file scanner for bib declarations |
| `internal/lsp/` | Minimal LSP server (JSON-RPC over stdio, cite-key completions) |

## Bib processing (post-compile)

After each successful compile, every registered `.bib` file is:
1. Parsed, canonical keys assigned (`{LastName}{Year}{Title}`)
2. Unseen entries validated via Crossref (DOI) or arXiv (eprint)
3. Fields normalised to allowed set; unknown fields dropped
4. Mandatory fields checked; warnings for missing ones
5. File rewritten only if changed

See `internal/bib/AGENT.md` for entry-type specs.

## Agent docs

| File | Scope |
|---|---|
| `cmd/root.agent.md` | Config struct (shared by commands) |
| `cmd/compile.agent.md` | `el compile` pass sequence |
| `cmd/config.agent.md` | `el config` flags |
| `cmd/init.agent.md` | `el init` steps |
| `cmd/bibentry.agent.md` | `el bibentry` ID handling |
| `cmd/parsebib.agent.md` | `el parsebib` cache allocation |
| `cmd/lsp.agent.md` | `el lsp` (thin, see below) |
| `internal/bib/AGENT.md` | Bib pipeline, entry types, validation, ISO 4 |
| `internal/lsp/AGENT.md` | LSP protocol, completion trigger, JSON-RPC |
| `internal/texscan/AGENT.md` | Tex scanner exports |
