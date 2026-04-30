# el init (`init.go`)

No flags. IEEEtran usage is detected from document class in main `.tex` file.

1. Scan current dir for `.tex` files containing `\begin{document}`
2. If multiple matches, prompt user to pick one
3. `os.MkdirAll(.el)`
4. `texscan.ResolveFileContents` — extract `\begin{filecontents}{*.bib}` blocks to disk
5. `texscan.HasDocumentClass(chosen, "IEEEtran")` — detect IEEEtran class
6. `texscan.FindBibFiles` to discover bib files referenced in chosen tex
7. `condenseBibFiles(bibFiles, dir, usesIEEEtran)` — consolidate all `.bib` files into at most two in project root:
   - Non-IEEEtran: entries → `bibliography.bib`; `@string`/`@preamble` → `preamble.bib`
   - IEEEtran:     entries → `bibliography.bib`; `@string`/`@preamble` → `IEEEabrv.bib`
   - Preamble file only created if non-empty; original files deleted
8. `texscan.RewriteBibReferences` — update `\bibliography`/`\addbibresource` in all tex files
9. Write `.el/config.json` (no special fields set; formatting applied auto at compile)
10. Append `.el` to `.git/info/exclude` (idempotent, no-op if no `.git`)
11. `bib.AllocateCacheEntries([entries bib file], ...)` — seed `.el/bib.json`; if renames returned, persist via `bib.SaveRenames` so first `el compile` rewrites `\cite{}` keys
