# el parsebib (`parsebib.go`)

Manually allocate/update bib cache entries from registered `.bib` files. Calls `bib.AllocateCacheEntries`:
- Parses all `.bib` files in config
- Deduplicates by DOI (Crossref validated), arXiv ID (arXiv validated), or canonical key (no-ID)
- Skips entries already in cache
- Does NOT rewrite bib files
- Prints count of newly cached entries

Useful for pre-validating bib entries without compiling, or re-populating cache after `.el/bib.json` deletion.
