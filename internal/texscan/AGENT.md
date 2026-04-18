# internal/texscan

Exports:

- `FindBibFiles(mainTex, dir string) []string` — scan `mainTex` and recursively included files for bib declarations (`\bibliography{}`, `\addbibresource{}`). Return dedup `.bib` names relative to `dir`. Append `.bib` if missing.
- `FindTexFiles(mainTex, dir string) []string` — return all `.tex` file paths (absolute) reachable from `mainTex` via `\input{}`/`\include{}`.
- `ResolveFileContents(mainTex, dir string) error` — find `\begin{filecontents[*]}{*.bib}...\end{filecontents[*]}` blocks in all reachable tex files, write embedded content to disk as `dir/name.bib`, remove block from tex file. Called by `el init` (before `FindBibFiles`) and `el compile` (before first pdflatex pass).
- `RewriteBibReferences(mainTex, dir string, newBibFiles []string) error` — rewrite `\bibliography{...}` and `\addbibresource{...}` in all reachable tex files. First occurrence per file replaced with new references; subsequent occurrences of same command type dropped.
- `StripComment(line string) string` — strip `%`-to-end-of-line comment.

All functions strip comments before pattern matching.
