# cmd

Cobra commands wired in `root.go`. Config struct (`Config`) holds `Main`, `AuxDir`, `BibFiles`, `AbbreviateJournals`; persisted as `.el.json`.

## el config (`config.go`)

Reads `.el.json`, updates the requested field, and rewrites it.

- `--abbreviate-journals=<true|false>` — enable/disable ISO 4 journal abbreviation (default: true, nil in JSON = true)

`AbbreviateJournals` is stored as `*bool` with `omitempty`; nil (absent) defaults to true. `cfg.abbreviateJournals()` is the helper used by compile.

## el init (`init.go`)

1. Scan cwd for `.tex` files containing `\begin{document}`; prompt if multiple
2. Create `.aux_dir/`
3. Call `texscan.FindBibFiles` to discover bib declarations
4. Write `.el.json`
5. Append `.aux_dir` and `.el.json` to `.git/info/exclude` if in a git repo

## el compile (`compile.go`)

Compile sequence:
1. First `pdflatex` pass — output buffered (bib warnings expected at this stage)
2. Detect bib tool from artifacts: `.bcf` → biber; `\bibdata` in `.aux` → bibtex; neither → skip
3. If bib tool found: run it, then second `pdflatex` pass; if output contains "rerun", run a third pass
4. Print filtered pdflatex output (errors, warnings, undefined refs, over/underfull boxes)
5. Create symlink `<stem>.pdf` → `.aux_dir/<stem>.pdf`
6. Call `bib.ProcessBibFiles` on all registered bib files
7. If `-o`/`--open`: open the PDF

Tool lookup: `exec.LookPath` first, then `/Library/TeX/texbin/` fallback (macOS TeX Live).

Bib file discovery fallback: if `BibFiles` is empty after init, `bibFilesFromArtifacts` reads them from `.aux`/`.bcf` artifacts and updates `.el.json`.
