# el init (`init.go`)

Flags: `--ieee` (bool, default false) — use IEEE bib file names and enable IEEE formatting.

1. Scan current dir for `.tex` files containing `\begin{document}`
2. If multiple matches, prompt user to pick one
3. `os.MkdirAll(.el)`
4. `texscan.ResolveFileContents` — extract `\begin{filecontents}{*.bib}` blocks to disk
5. `texscan.FindBibFiles` to discover bib files referenced in chosen tex
6. `condenseBibFiles(bibFiles, dir, ieee)` — consolidate all `.bib` files into at most two in project root:
   - Without `--ieee`: entries → `bibliography.bib`; `@string`/`@preamble` → `preamble.bib`
   - With `--ieee`:    entries → `bibliography.bib`; `@string`/`@preamble` → `IEEEabrv.bib`
   - Preamble file only created if non-empty; original files deleted
7. `texscan.RewriteBibReferences` — update `\bibliography`/`\addbibresource` in all tex files
8. Write `.el/config.json` (sets `ieee_format: true` when `--ieee` given)
9. Append `.el` to `.git/info/exclude` (idempotent, no-op if no `.git`)
10. `bib.AllocateCacheEntries([entries bib file], ...)` — seed `.el/bib.json`; if renames returned, persist via `bib.SaveRenames` so first `el compile` rewrites `\cite{}` keys
