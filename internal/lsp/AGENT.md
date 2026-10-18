# internal/lsp

Minimal LSP server for `el lsp`. Hand-rolled JSON-RPC over stdio, no external LSP library.

Surfaces:
- Cite-key completion (read from the global bib cache — see `internal/bib/global.go`).
- Cite completion sets `insertText = key+"}"` when pedantic check `no-block-citations` is enabled and no `}` follows the cursor (cursor lands after the brace).
- Label completion for `\ref`-like commands (`labels.go`): `\label{}` keys from the open buffer + files reachable via `\input`/`\include` (disk, or open buffer if tracked). `\eqref` only offers labels starting with `equation:`. Recomputed per request. Commands: `ref eqref autoref cref Cref vref Vref pageref nameref labelcref cpageref Cpageref` (starred/optional args ok). Checked before cite detection in `complete()`.
- Env table refresh on `didSave` (`envs.go`): rescans `.tex`/`.sty`/`.cls` under the nearest `.el` project root and updates only declaration-derived parts of `.el/envs.json` (theorem flag, `\newenvironment`/`\NewDocumentEnvironment` arg counts) via `texscan.ScanEnvDecls`/`ApplyEnvDecls`, plus declared command arg counts in `.el/cmds.json` (`ScanCmdDecls`). Usage-derived arg counts stay compile-gated.
- Static pedantic diagnostics (push on didOpen / didChange).
- Spell-check diagnostics (push on didOpen / didChange) when `cfg.SpellLang` set.
- Code actions (autofix) for fixable static checks. Spell-check produces diagnostics only — no quickfix.

## Entry point

`Serve(cfg Config, r io.Reader, w io.Writer) error` — called from `cmd/lsp.go`. `Config` carries cite-key items, the list of enabled static check names, and optional spell-check wiring (`SpellLang`, `SpellAuxDir`, `SpellPaths`). When `SpellLang != ""`, a long-lived `*spell.Session` is opened and reused across every `didChange`; it is closed when `Serve` returns.

## Files

| File | Contents |
|---|---|
| `protocol.go` | LSP wire types: capabilities, completion, diagnostics, code action, workspace edit |
| `position.go` | `byteToUTF16(line, byteOff)` — convert byte offset to LSP-spec UTF-16 code units |
| `index.go` | `BuildItems()` — reads keys from the global bib cache |
| `labels.go` | `detectRefPrefix`, `docLabels`, `completeRef` — label completion |
| `server.go` | JSON-RPC loop, dispatch, completion, diagnostics, code action handlers |

## Protocol subset

| Method | Action |
|---|---|
| `initialize` | Capabilities: `textDocumentSync={openClose, change=1 (full), save}`, completionProvider with `{` `,` triggers, codeActionProvider always advertised (returns null when no checks enabled, avoiding client "method not supported" errors) |
| `initialized` | Ignore |
| `textDocument/didOpen` | Store doc; `publishDiagnostics` |
| `textDocument/didSave` | `refreshEnvTable(uri)` |
| `textDocument/didChange` | Update doc (full sync); `publishDiagnostics` |
| `textDocument/completion` | Cite-key completion (see below) |
| `textDocument/codeAction` | One "Apply pedantic autofix" quickfix when fixable issues exist; whole-document text edit |
| `shutdown` | Reply null, set shuttingDown |
| `exit` | `os.Exit(0)` if shuttingDown, else `os.Exit(1)` |
| unknown request | MethodNotFound -32601 |
| unknown notification | Ignore |

## Diagnostics

`publishDiagnostics(uri)` runs:

1. `pedantic.RunSourceChecksText` → `lspDiagnostic` per finding (severity=Warning, source=`el-pedantic`, range = whole line).
2. `s.spellSess.Check` (if spell enabled) → `lspDiagnostic` per misspelling (severity=Information, source=`el-spell`, range = word — `Col` and `len(Word)` mapped through `byteToUTF16` against the raw line).

Always pushes a single combined list — empty list clears stale state. Spell column offsets may drift on lines containing TeX accent macros because `spell.NormalizeAccents` is not length-preserving (known limit, shared with the CLI spell pass).

Before each spell pass the server calls `spellSess.Stale()`; if any tracked dict / ignore file changed on disk (e.g. via `el spell add` in another terminal), it `Reopen()`s the session so the next diagnostics reflect the new wordlist. Stat cost is six files per lint.

## Code actions

`codeActions(p)` runs `pedantic.RunSourceFixesText` (pure, no disk I/O) on the in-memory doc. If anything changed, returns one quickfix action with a single whole-document `TextEdit`. The editor applies the edit; the next `didChange` re-lints and clears the diagnostic.

## Completion trigger detection (`server.go: detectCitePrefix`)

Regex on line up to cursor matches cite commands then `{[^}]*$`. Supported commands: `\cite`, `\citet`, `\citep`, `\citealt`, `\citealp`, `\citeauthor`, `\citeyear`, `\citeyearpar`, `\citenum`, capitalised `\Cite*` variants, starred forms (`\citet*`, etc.), and optional `[...]` arguments before the brace.

## Bib key loading (`index.go: BuildItems`)

Reads all keys from the global bib cache via `bib.LoadCacheKeys()`. No bib file parsing. Loaded once at startup — **restart LSP to pick up new entries**.

## Known limitations

- `complete()` interprets the client's `Position.Character` as a byte offset rather than a UTF-16 unit. ASCII-safe for cite-command lines; multi-byte chars before the cursor on the same line could misalign prefix detection. Outbound positions (diagnostics, code-action ranges) ARE converted via `byteToUTF16`.
- Code action returns whole-document edit. Editors handle this but cursor position may shift on non-trivial diffs.
- No incremental sync — every change re-lints the whole document. Cheap for typical project sizes.
- Spell column offsets drift slightly on lines containing TeX accent macros (`NormalizeAccents` is not length-preserving).
