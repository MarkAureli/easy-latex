# el compile (`compile.go`)

Pass sequence:
1. **Hash check** — if `bibliography.bib` changed since last compile/parsebib (`bib.BibFileChanged`), auto-run `bib.AllocateCacheEntries`, save renames to `.el/renames.json`, update hash
2. **Pass 1** — `runPdflatex`; buffer output (bib warnings expected here)
3. **Bib file discovery fallback** — if `cfg.BibFiles` empty after pass 1, parse `.aux`/`.bcf` to find them, save to config
4. **Cite-key rewrite** — if `.el/renames.json` non-empty (`bib.LoadRenames`), rewrite `\cite{}` in all `.tex` files via `rewriteCiteKeys`, clear renames, re-run pdflatex
5. **Write bibliography** — `bib.WriteBibFromCache`: extract cited keys from `.aux`/`.bcf` (`citedKeysFromArtifacts`), write only those entries to `bibliography.bib` with all config transforms applied; update hash
6. **Detect bib tool** — `.bcf` present → `biber`; `.aux` contains `\bibdata{` → `bibtex`; else none
7. **Bib pass** — `runBibTool`; biber uses `--input/output-directory`; bibtex runs from inside aux dir with `BIBINPUTS=..:`
8. **Pass 2** — `runPdflatex`; print filtered output
9. **Pass 3** — if pass 2 output contains "rerun", run once more

Post-compile: remove stale symlink, create `<stem>.pdf → .el/<stem>.pdf`.

Flags: `--open` / `-o` — call `open <pdf>` after success.

Output filtering (`filterLines`): keeps lines matching `^!`, `^l.\d+`, warning, error, undefined, multiply defined, Over/Underfull.

`entriesBibFile(bibFiles)` — returns path of `bibliography.bib` from config bib list, or `""` if absent.
