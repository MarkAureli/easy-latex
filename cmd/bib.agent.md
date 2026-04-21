# el bib (`bib.go`)

Command group for bibliography management.

## Subcommands

### `el bib list`

Lists all cached bib entries from `.el/bib.json`. Uses `bib.LoadCacheEntries(auxDir)` returning `[]CacheEntryInfo`. Displays truncated table: key, type, author, title. Helper funcs `truncate`, `truncateAuthor` for column width.

### `el bib add <ID>`

Add a single entry to bib cache from a bare DOI or arXiv ID. Implemented by `runBibAdd`. Calls `bib.AddEntryFromID(id, auxDir, log)` which returns `(key, isNew, err)`.

- `isNew=true` — announces newly added entry with key
- `isNew=false` — announces entry already cached with existing key
- `bib.ErrUnrecognizedID` — warns unrecognized format, exits 0

Uses `bibLogger` (`cmd/biblog.go`) for colored output. No config load required.

## Related

- `cmd/bibentry.go` — hidden alias, delegates to `runBibAdd`
- `cmd/biblog.go` — `bibLogger` implements `bib.Logger` with colored output via `internal/term`
