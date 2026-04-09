# cmd

Cobra commands wired in `root.go`. Config struct (`Config`) holds `Main`, `AuxDir`, `BibFiles`, `AbbreviateJournals`, `BraceTitles`, `IEEEFormat`; persisted as `.el.json`.

## el config (`config.go`)

Reads `.el.json`, updates the requested field(s), and rewrites it. Any combination of flags may be passed in a single invocation.

| Flag | Default | nil meaning |
|---|---|---|
| `--abbreviate-journals=<bool>` | true | true |
| `--brace-titles=<bool>` | false | false |
| `--ieee-format=<bool>` | false | false |

All options stored as `*bool` with `omitempty`. Helpers: `cfg.abbreviateJournals()`, `cfg.braceTitles()`, `cfg.ieeeFormat()`.

`--ieee-format=true` implies `brace-titles=true` and converts `@misc` arXiv entries to `@unpublished` (see bib CLAUDE.md).

## el init (`init.go`)

1. Scan cwd for `.tex` files containing `\begin{document}`; prompt if multiple
2. Create `.aux_dir/`
3. Call `texscan.FindBibFiles` to discover bib declarations
4. Write `.el.json`
5. Append `.aux_dir` and `.el.json` to `.git/info/exclude` if in a git repo

## el compile (`compile.go`)

Compile sequence:
1. First `pdflatex` pass — output buffered (bib warnings expected at this stage)
2. Discover bib files from artifacts if `BibFiles` is empty
3. Call `bib.ProcessBibFiles` — normalise bib files before the bib tool runs
4. Detect bib tool from artifacts: `.bcf` → biber; `\bibdata` in `.aux` → bibtex; neither → skip
5. If bib tool found: run it, then second `pdflatex` pass; if output contains "rerun", run a third pass
6. Print filtered pdflatex output (errors, warnings, undefined refs, over/underfull boxes)
7. Create symlink `<stem>.pdf` → `.aux_dir/<stem>.pdf`
8. If `-o`/`--open`: open the PDF

Tool lookup: `exec.LookPath` first, then `/Library/TeX/texbin/` fallback (macOS TeX Live).

Bib file discovery fallback: if `BibFiles` is empty after init, `bibFilesFromArtifacts` reads them from `.aux`/`.bcf` artifacts and updates `.el.json`.
