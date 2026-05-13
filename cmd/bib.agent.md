# el bib (`bib.go`)

Command group for bibliography management.

## Subcommands

### `el bib list`

Lists cached bib entries from `.el/bib.json`. Uses `bib.LoadCacheEntries(auxDir)` returning `[]CacheEntryInfo`. Displays truncated table: key, type, source. Helper funcs `truncate`, `truncateAuthor` for column width.

When config exists with `main` tex file, scans for `\cite{}` keys via `texscan.FindCiteKeys` and groups output into "Referenced" and "Unreferenced" sections. Falls back to flat list when config unavailable.

Flags:
- `--cited` — show only entries referenced in `.tex` files
- `--uncited` — show only entries not referenced in `.tex` files

### `el bib parse`

Manually allocate/update bib cache entries from registered `.bib` files. Calls `bib.AllocateCacheEntries(bibFiles, auxDir, log)`:
- Parses all `.bib` files in config
- Batch-prefetches all uncached arXiv ids in one API call (`bib.PrefetchArxivIDs`) before per-entry validation
- Deduplicates by DOI (Crossref validated), arXiv ID (arXiv validated), or canonical key (no-ID)
- Skips entries already in cache
- Does NOT rewrite bib files
- Prints count of newly cached entries
- Announces key renames via `bib.Logger` (old key -> new canonical key)

Uses `bibLogger` (`cmd/biblog.go`) for colored output.

Useful for pre-validating bib entries without compiling, or re-populating cache after `.el/bib.json` deletion.

### `el bib add <ID> [<ID>...]`

Add one or more entries to bib cache from bare DOIs or arXiv IDs (`cobra.MinimumNArgs(1)`). Implemented by `runBibAdd`. Pre-batches all arXiv ids in the argument list via `bib.PrefetchArxivIDs` (single API call) then loops `bib.AddEntryFromID(id, auxDir, log)` for each arg.

Per-arg outcomes:
- `isNew=true` — announces newly added entry with key
- `isNew=false` — announces entry already cached with existing key
- `bib.ErrUnrecognizedID` — warns unrecognized format, continues to next arg
- Other errors — prints to stderr, continues; first error returned at end

Uses `bibLogger` (`cmd/biblog.go`) for colored output. No config load required.

### `el bib remove <key>`

Remove a single entry from bib cache by key. Implemented by `runBibRemove`. Calls `bib.RemoveEntryFromCache(key, auxDir)` which returns `(removed bool, err error)`.

- Key completion via `bibKeyCompletion` — shows all keys in cache
- Warns if key not found, exits 0
- No config load required

## Related

- `cmd/biblog.go` — `bibLogger` implements `bib.Logger` with colored output via `internal/term`
