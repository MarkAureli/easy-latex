# el bib (`bib.go`)

Command group for bibliography management. All subcommands read/write the **global bib cache** (`bib.GlobalBibPath()`), not any per-project cache file.

## Subcommands

### `el bib list`

Lists entries from the global bib cache. Uses `bib.LoadCacheEntries()` returning `[]CacheEntryInfo`. Displays truncated table: key, type, source. Helper funcs `truncate`, `truncateAuthor` for column width.

When config exists with `main` tex file, scans for `\cite{}` keys via `texscan.FindCiteKeys` and groups output into "Referenced" and "Unreferenced" sections. Falls back to flat list when config unavailable.

Flags:
- `--cited` — show only entries referenced in `.tex` files
- `--uncited` — show only entries not referenced in `.tex` files
- `--search <q>` — case-insensitive substring filter on key, first author, and title (composes with `--cited` / `--uncited` and section grouping)

### `el bib parse`

Manually allocate/update bib cache entries from registered `.bib` files. Calls `bib.AllocateCacheEntries(bibFiles, retryTimeout, log)`:
- Parses all `.bib` files in config
- Batch-prefetches all uncached arXiv ids in one API call (`bib.PrefetchArxivIDs`) before per-entry validation
- Deduplicates by DOI (Crossref validated), arXiv ID (arXiv validated), or canonical key (no-ID)
- Skips entries already in cache
- Does NOT rewrite bib files
- Prints count of newly cached entries
- Announces key renames via `bib.Logger` (old key -> new canonical key)

Uses `bibLogger` (`cmd/biblog.go`) for colored output.

Useful for pre-validating bib entries without compiling, or re-populating the global cache after manual deletion.

### `el bib add <ID> [<ID>...]`

Add one or more entries to the global bib cache from bare DOIs or arXiv IDs (`cobra.MinimumNArgs(1)`). Implemented by `runBibAdd`. Pre-batches all arXiv ids in the argument list via `bib.PrefetchArxivIDs` (single API call) then loops `bib.AddEntryFromID(id, log)` for each arg.

Per-arg outcomes:
- `isNew=true` — announces newly added entry with key
- `isNew=false` — announces entry already cached with existing key
- `bib.ErrUnrecognizedID` — warns unrecognized format, continues to next arg
- Other errors — prints to stderr, continues; first error returned at end

Uses `bibLogger` (`cmd/biblog.go`) for colored output. No config load required.

### `el bib remove <key> [<key>...]`

Remove one or more entries from the global bib cache by key (`cobra.MinimumNArgs(1)`). Implemented by `runBibRemove`. Calls `bib.RemoveEntriesFromCache(keys)` which loads/saves the cache once under `withGlobalLock` and returns `(removed, notFound []string, err error)`. Prints one "Removed" line per success and a "not found" warning per missing key.

- Key completion via `bibKeyCompletion` — shows all keys in cache
- Warns if key not found, exits 0
- No config load required

## Related

- `cmd/biblog.go` — `bibLogger` implements `bib.Logger` with colored output via `internal/term`
