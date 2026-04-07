# internal/texscan

Single exported function: `FindBibFiles(mainTex, dir string) []string`

Recursively scans `mainTex` and any files pulled in via `\input{}`/`\include{}` for bibliography declarations:

- `\bibliography{a,b}` — bibtex style
- `\addbibresource{refs.bib}` — biblatex style

Returns deduplicated `.bib` file names relative to `dir`. Appends `.bib` extension if absent. Comments (`%` to end of line) are stripped before pattern matching.
