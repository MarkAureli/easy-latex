# internal/texscan

Exports:

- `FindBibFiles(mainTex, dir string) []string` — scan `mainTex` and recursively included files for bib declarations (`\bibliography{}`, `\addbibresource{}`). Return dedup `.bib` names relative to `dir`. Append `.bib` if missing.
- `FindTexFiles(mainTex, dir string) []string` — return all `.tex` file paths reachable from `mainTex` via `\input{}`/`\include{}` (relative when `dir` is relative).
- `FindCiteKeys(mainTex, dir string) []string` — scan all reachable `.tex` files for `\cite{}`, `\citep{}`, `\citet{}`, `\citeauthor{}`, `\parencite{}`, `\textcite{}`, `\autocite{}`, `\fullcite{}` and variants; return sorted, deduplicated keys. Strips comments before matching. Optional citation arguments `[...]` are skipped.
- `ResolveFileContents(mainTex, dir string) error` — find `\begin{filecontents[*]}{*.bib}...\end{filecontents[*]}` blocks in all reachable tex files, write embedded content to disk as `dir/name.bib`, remove block from tex file. Called by `el init` (before `FindBibFiles`).
- `RewriteBibReferences(mainTex, dir string, newBibFiles []string) error` — rewrite `\bibliography{...}` and `\addbibresource{...}` in all reachable tex files. First occurrence per file replaced with new references; subsequent occurrences of same command type dropped.
- `StripComment(line string) string` — strip `%`-to-end-of-line comment (escaped `\%` preserved).

All functions strip comments before pattern matching.
