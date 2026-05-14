# easy-latex

CLI tool (`el`) for compiling LaTeX docs. Go project, module `github.com/MarkAureli/easy-latex`.

## Key files

| Path | Role |
|---|---|
| `.el/` | Per-project working directory: config, all pdflatex/bibtex/biber intermediates, bib hash, renames |
| `.el/config.json` | Config: main tex file, aux dir, bib paths, processing options |
| Global bib cache | Per-user validation cache shared across all projects. Path via `bib.GlobalBibPath()`: `$EL_GLOBAL_BIB` → `$XDG_DATA_HOME/easy-latex/bib.json` → mac `~/Library/Application Support/easy-latex/bib.json` → linux `~/.local/share/easy-latex/bib.json`. Read-modify-write is serialised via `flock` on a sibling `.lock` file; writes are atomic via tmp+rename. |
| `cmd/` | Cobra commands (`bib`, `check`, `compile`, `config`, `init`, `lsp`) |
| `internal/bib/` | Bib parsing, key gen, formatting, validation, Logger interface, retry logic |
| `internal/term/` | Shared terminal detection (`IsTerminal`) + ANSI color codes (`Colors` struct, `Detect()`) |
| `internal/texscan/` | Tex file scanner for bib declarations |
| `internal/pedantic/` | Pedantic checks: source-level + post-compile (pdfsavepos) |
| `internal/spell/` | Hunspell-backed spell-check (en_GB / en_US) with layered dicts and ignore lists |
| `internal/lsp/` | Minimal LSP server (JSON-RPC over stdio, cite-key completions) |

## Bib processing

Two-phase design: **cache allocation** and **bib generation from cache**. All validated entries live in the **global bib cache** (path from `bib.GlobalBibPath()`); there is no per-project cache file. Read-modify-write paths wrap their work in `withGlobalLock` for cross-process safety.

- `AllocateCacheEntries(bibFiles, retryTimeout, log Logger)` — parses bib files, assigns canonical keys (`{LastName}{Year}{Title}`), validates unseen entries via Crossref (DOI) or arXiv (eprint), seeds the global cache. Used by `el init`, `el bib parse`, and auto-triggered by `el compile` when `bibliography.bib` changes.
- `WriteBibFromCache(path, citeKeys, opts)` — reconstructs entries from the global cache for cited keys only, applies config transforms (journal abbreviation, author formatting, brace titles, etc.), writes `bibliography.bib`. Called by `el compile` after pass 1. Any cited key present in the global cache but not yet in the project bib file is materialised here (auto-import on compile).
- `AddEntryFromID(id, log Logger) (key, isNew, err)` — single-entry insertion from bare DOI/arXiv ID. Used by `el bib add`.

### Logger architecture

`bib.Logger` interface (`logger.go`): `Info`, `Warn`, `Progress` methods. All `[bib]` warnings go to stderr, info/success to stdout. Commands provide their own implementation (e.g. `cmd/biblog.go` — `bibLogger` with colored output via `internal/term`). `nopLogger` used when no output desired.

### HTTP retry

`doWithRetry` (`retry.go`): exponential backoff (1s/2s/4s, max 3 attempts), retries on 429/5xx/timeouts. `friendlyHTTPError` converts HTTP status codes to human-readable messages. Progress messages shown via Logger ("fetching metadata from Crossref/arXiv…").

See `internal/bib/AGENT.md` for entry-type specs.

## Agent docs

| File | Scope |
|---|---|
| `cmd/root.agent.md` | Config struct (shared by commands) |
| `cmd/bib.agent.md` | `el bib` command group (`list`, `add`, `parse`) |
| `cmd/check.agent.md` | `el check` static-only linter + autofix |
| `cmd/compile.agent.md` | `el compile` pass sequence |
| `cmd/config.agent.md` | `el config` flags |
| `cmd/init.agent.md` | `el init` steps |
| `cmd/lsp.agent.md` | `el lsp` (thin, see below) |
| `cmd/spell.agent.md` | `el spell` add/remove/list dict and ignore files |
| `internal/bib/AGENT.md` | Bib pipeline, entry types, validation, ISO 4 |
| `internal/lsp/AGENT.md` | LSP protocol, completion trigger, JSON-RPC |
| `internal/pedantic/AGENT.md` | Pedantic check system |
| `internal/spell/AGENT.md` | Spell-check engine (hunspell pipe-mode, layered dicts) |
| `internal/texscan/AGENT.md` | Tex scanner exports (incl. `ProseRuns`) |
