# el compile (`compile.go`)

Pass sequence:
1. **Hash check** — if `bibliography.bib` changed since last compile/bib parse (`bib.BibFileChanged`), announces "bibliography.bib changed, re-parsing…", auto-runs `bib.AllocateCacheEntries`, save renames to `.el/renames.json`, update hash
2. **Pass 1** — `runPdflatex`; buffer output (bib warnings expected here)
3. **Bib file discovery fallback** — if `cfg.BibFiles` empty after pass 1, parse `.aux`/`.bcf` to find them, save to config
4. **Cite-key rewrite** — if `.el/renames.json` non-empty (`bib.LoadRenames`), announces key renames and `.tex` file rewrites, rewrite `\cite{}` in all `.tex` files via `rewriteCiteKeys`, clear renames, re-run pdflatex
5. **Write bibliography** — `bib.WriteBibFromCache`: extract cited keys from `.aux`/`.bcf` (`citedKeysFromArtifacts`), filter out keys defined in generated `.bib` files (`generatedBibKeys` — any `\bibdata{}` source not in `cfg.BibFiles`, e.g. revtex `citeautoscript`'s `<stem>Notes.bib`), write only those entries to `bibliography.bib` with all config transforms applied; update hash. Cited keys present in the global bib cache but missing from the project bib file are materialised here (auto-import on compile).
6. **Detect bib tool** — `.bcf` present → `biber`; `.aux` contains `\bibdata{` → `bibtex`; else none
7. **Bib pass** — `runBibTool`; biber uses `--input/output-directory`; bibtex runs from inside aux dir with `BIBINPUTS=..:` and `BSTINPUTS=..:` (both needed so bibtex finds `.bib` and `.bst` files in project root)
8. **Pass 2** — `runPdflatex`; print filtered output
9. **Pass 3–4** — up to 2 additional passes if output contains "rerun" (`for range 2` loop), stabilizing cross-references and citations

Post-compile: copy `<stem>.pdf` from `.el/` to project root. If checks enabled: run `pedantic.RunSourceChecks` + `pedantic.RunPostCompileChecks` + spell-check. With `--fix`, the source-phase detector runs both before and after `pedantic.RunSourceFixes`; the pre/post diff goes into a `Pedantics (fixed)` section (default colour, bold header), and the post-fix diagnostics + post-compile diagnostics go into `Pedantics (remaining)` (yellow). Output sections, in order with blank line between non-empty sections: `Pedantics (fixed)` (when `--fix`), `Pedantics`/`Pedantics (remaining)`, `Misspellings`, `Warnings` (compile-time pdflatex warnings, yellow). Followed by a yellow summary `N pedantic(s), M misspelling(s), K warning(s)` — fixed pedantics are excluded from the count. `Compiled successfully -> <pdf>` always prints when PDF made. Strict mode (config `strict: true` or `--strict`) returns `errStrict` after printing if remaining pedantics, misspellings, or compile warnings are non-zero (fixed pedantics never trigger). When any post-compile check is enabled: writes each enabled check's embedded `.sty` (`pedantic.PostCompileStys`) to `.el/`, injects via `\RequirePackage{<pkg>}…\input{main.tex}` (sorted package order) with `TEXINPUTS` pointing to aux dir and `-jobname=<stem>`. Currently: `no-math-linebreak` → `el-mathpos.sty`; `no-section-linebreak` → `el-sectionpos.sty`; `no-orphan-line` → `el-orphan.sty`.

Uses `internal/term` for ANSI colors (replaces inline color vars). Uses `bibLogger` (`cmd/biblog.go`) for bib operation messages.

Flags:
- `--open` / `-o` — call `open <pdf>` after success.
- `--fix` — apply `pedantic.RunSourceFixes` to source files before running checks (static-with-fix checks only); reports modified files. Mutually exclusive with `--no-check`.
- `--no-check` — skip all pedantic checks for this run (static and dynamic); also skips writing any post-compile `.sty` and `\RequirePackage` instrumentation. Mutually exclusive with `--fix`.
- `--strict` / `--no-strict` — override `cfg.Strict` for this run. In strict mode any pedantic, spelling, or compile-time warning makes runCompile return a non-zero error after printing all sections and "Compiled successfully".

Output filtering (`filterLines`): keeps lines matching `^!`, `^l.\d+`, warning, error, undefined, multiply defined, Over/Underfull.

`entriesBibFile(bibFiles)` — returns path of `bibliography.bib` from config bib list, or `""` if absent.
