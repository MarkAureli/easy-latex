# el compile (`compile.go`)

Pass sequence:
1. **Hash check** — if `bibliography.bib` changed since last compile/bib parse (`bib.BibFileChanged`), announces "bibliography.bib changed, re-parsing…", auto-runs `bib.AllocateCacheEntries`, save renames to `.el/renames.json`, update hash
2. **Pass 1** — `runPdflatex`; buffer output (bib warnings expected here)
3. **Bib file discovery fallback** — if `cfg.BibFiles` empty after pass 1, parse `.aux`/`.bcf` to find them, save to config
4. **Cite-key rewrite** — if `.el/renames.json` non-empty (`bib.LoadRenames`), announces key renames and `.tex` file rewrites, rewrite `\cite{}` in all `.tex` files via `rewriteCiteKeys`, clear renames, re-run pdflatex
5. **Write bibliography** — `bib.WriteBibFromCache`: extract cited keys from `.aux`/`.bcf` (`citedKeysFromArtifacts`), filter out keys defined in generated `.bib` files (`generatedBibKeys` — any `\bibdata{}` source not in `cfg.BibFiles`, e.g. revtex `citeautoscript`'s `<stem>Notes.bib`), write only those entries to `bibliography.bib` with all config transforms applied; update hash
6. **Detect bib tool** — `.bcf` present → `biber`; `.aux` contains `\bibdata{` → `bibtex`; else none
7. **Bib pass** — `runBibTool`; biber uses `--input/output-directory`; bibtex runs from inside aux dir with `BIBINPUTS=..:` and `BSTINPUTS=..:` (both needed so bibtex finds `.bib` and `.bst` files in project root)
8. **Pass 2** — `runPdflatex`; print filtered output
9. **Pass 3–4** — up to 2 additional passes if output contains "rerun" (`for range 2` loop), stabilizing cross-references and citations

Post-compile: copy `<stem>.pdf` from `.el/` to project root. If checks enabled: run `pedantic.RunSourceChecks` + `pedantic.RunPostCompileChecks` + spell-check. Output, in order with blank line between non-empty sections, all yellow/bold: `Pedantics:`, `Misspellings:`, `Warnings:` (compile-time pdflatex warnings), followed by a yellow summary `N pedantic(s), M misspelling(s), K warning(s)` (each term singular when its count is 1). `Compiled successfully -> <pdf>` line always prints when PDF made. If strict mode (config `strict: true` or `--strict`) and any non-empty section, runCompile returns error after printing summary. When `no-math-linebreak` is enabled: writes embedded `el-mathpos.sty` to `.el/`, injects via `\RequirePackage{el-mathpos}\input{main.tex}` with `TEXINPUTS` pointing to aux dir and `-jobname=<stem>`.

Uses `internal/term` for ANSI colors (replaces inline color vars). Uses `bibLogger` (`cmd/biblog.go`) for bib operation messages.

Flags:
- `--open` / `-o` — call `open <pdf>` after success.
- `--fix` — apply `pedantic.RunSourceFixes` to source files before running checks (static-with-fix checks only); reports modified files. Mutually exclusive with `--no-check`.
- `--no-check` — skip all pedantic checks for this run (static and dynamic); also skips writing `el-mathpos.sty` and `needsMathPos` instrumentation. Mutually exclusive with `--fix`.
- `--strict` / `--no-strict` — override `cfg.Strict` for this run. In strict mode any pedantic, spelling, or compile-time warning makes runCompile return a non-zero error after printing all sections and "Compiled successfully".

Output filtering (`filterLines`): keeps lines matching `^!`, `^l.\d+`, warning, error, undefined, multiply defined, Over/Underfull.

`entriesBibFile(bibFiles)` — returns path of `bibliography.bib` from config bib list, or `""` if absent.
