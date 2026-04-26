# internal/lsp

Minimal LSP server for `el lsp`. Hand-rolled JSON-RPC over stdio, no external LSP library.

Surfaces:
- Cite-key completion (read from `.el/bib.json`).
- Static pedantic diagnostics (push on didOpen / didChange).
- Code actions (autofix) for fixable static checks.

## Entry point

`Serve(cfg Config, r io.Reader, w io.Writer) error` — called from `cmd/lsp.go`. `Config` carries cite-key items and the list of enabled static check names.

## Files

| File | Contents |
|---|---|
| `protocol.go` | LSP wire types: capabilities, completion, diagnostics, code action, workspace edit |
| `index.go` | `BuildItems(auxDir)` — reads keys from `.el/bib.json` |
| `server.go` | JSON-RPC loop, dispatch, completion, diagnostics, code action handlers |

## Protocol subset

| Method | Action |
|---|---|
| `initialize` | Capabilities: `textDocumentSync=1`, completionProvider with `{` `,` triggers, codeActionProvider when ≥1 check enabled |
| `initialized` | Ignore |
| `textDocument/didOpen` | Store doc; `publishDiagnostics` |
| `textDocument/didChange` | Update doc (full sync); `publishDiagnostics` |
| `textDocument/completion` | Cite-key completion (see below) |
| `textDocument/codeAction` | One "Apply pedantic autofix" quickfix when fixable issues exist; whole-document text edit |
| `shutdown` | Reply null, set shuttingDown |
| `exit` | `os.Exit(0)` if shuttingDown, else `os.Exit(1)` |
| unknown request | MethodNotFound -32601 |
| unknown notification | Ignore |

## Diagnostics

`publishDiagnostics(uri)` runs `pedantic.RunSourceChecksText` over the in-memory doc, maps each `pedantic.Diagnostic` to an `lspDiagnostic` (severity=Warning, source=`el-pedantic`, range = full line). Always pushes — empty list clears stale state.

## Code actions

`codeActions(p)` runs `pedantic.RunSourceFixesText` (pure, no disk I/O) on the in-memory doc. If anything changed, returns one quickfix action with a single whole-document `TextEdit`. The editor applies the edit; the next `didChange` re-lints and clears the diagnostic.

## Completion trigger detection (`server.go: detectCitePrefix`)

Regex on line up to cursor matches cite commands then `{[^}]*$`. Supported commands: `\cite`, `\citet`, `\citep`, `\citealt`, `\citealp`, `\citeauthor`, `\citeyear`, `\citeyearpar`, `\citenum`, capitalised `\Cite*` variants, starred forms (`\citet*`, etc.), and optional `[...]` arguments before the brace.

## Bib key loading (`index.go: BuildItems`)

Reads all keys from `.el/bib.json` via `bib.LoadCacheKeys(auxDir)`. No bib file parsing. Loaded once at startup — **restart LSP to pick up new entries**.

## Known limitations

- LSP positions are byte offsets, not UTF-16 code units. ASCII-safe; multi-byte characters may misposition diagnostics.
- Code action returns whole-document edit. Editors handle this but cursor position may shift on non-trivial diffs.
- No incremental sync — every change re-lints the whole document. Cheap for typical project sizes.
