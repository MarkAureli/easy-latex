# internal/lsp

Minimal LSP server for `el lsp`. Cite-key completions only. No external LSP library — hand-rolled JSON-RPC over stdio (~400 lines).

## Entry point

`Serve(items []completionItem, r io.Reader, w io.Writer) error` — called from `cmd/lsp.go` with pre-built items and `os.Stdin`/`os.Stdout`.

## Files

| File | Contents |
|---|---|
| `protocol.go` | Minimal LSP wire types (request, response, capabilities, completion) |
| `index.go` | `BuildItems(auxDir string) []completionItem` — reads keys from `.el/bib.json` |
| `server.go` | JSON-RPC loop, message dispatch, trigger detection, completion filtering |

## Protocol subset

Only these messages handled:

| Method | Action |
|---|---|
| `initialize` | Reply with capabilities: `textDocumentSync=1` (full), completionProvider trigger chars `{` `,` |
| `initialized` | Ignore |
| `textDocument/didOpen` | Store full doc text by URI |
| `textDocument/didChange` | Update doc text (full sync, last change wins) |
| `textDocument/completion` | Detect cite trigger, filter items, respond |
| `shutdown` | Reply null, set shuttingDown flag |
| `exit` | `os.Exit(0)` if shuttingDown, else `os.Exit(1)` |
| unknown request (has ID) | MethodNotFound -32601 |
| unknown notification | Ignore |

## Completion trigger detection (`server.go: detectCitePrefix`)

Regex `\\cite[tp]?\{[^}]*$` on line up to cursor. If match, partial key = text after last `{` or `,` (trimmed). Empty prefix = return all items. Non-empty prefix = filter by `strings.HasPrefix`.

Supported commands: `\cite`, `\citet`, `\citep`.

## Bib key loading (`index.go: BuildItems`)

Reads all keys from `.el/bib.json` via `bib.LoadCacheKeys(auxDir)`. No bib file parsing. Loaded once at startup — **restart LSP to pick up new entries**.

## completionItem

Only `label` (cite key) set. `kind` omitted (avoids editor annotation). `detail`/`documentation` unused.
