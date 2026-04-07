# easy-latex

A minimal CLI tool for compiling LaTeX documents without the noise.

## Commands

### `el init`

Run in a folder containing a `.tex` file with `\begin{document}`. Detects the main file, creates an `.aux_dir/` for auxiliary files, and saves the configuration to `.el.json`.

```
$ el init
Initialized. Main file: thesis.tex
Bib files: refs.bib
```

If multiple eligible `.tex` files are found, you will be prompted to pick one.

`el init` also scans the main file (and any files pulled in via `\input{}`/`\include{}`) for bibliography declarations — both `\bibliography{}` (bibtex) and `\addbibresource{}` (biblatex) — and stores the discovered `.bib` file paths in `.el.json`.

If the project is a git repository, `el init` also appends `.aux_dir` and `.el.json` to `.git/info/exclude` (local-only gitignore) so generated files are never accidentally committed.

### `el compile`

Compiles the document using `pdflatex`. Only warnings and errors are printed — all other LaTeX output is suppressed. On success, a symlink to the PDF is created in the project root.

```
$ el compile
Compiled successfully -> thesis.pdf
```

If the document uses a bibliography, `el` automatically detects the required tool and runs the full compilation sequence:

- `\bibliography{}` (natbib, plain bibtex) → `pdflatex` → `bibtex` → `pdflatex`
- `\usepackage{biblatex}` → `pdflatex` → `biber` → `pdflatex`

Detection is based on the auxiliary files produced by the first `pdflatex` pass (`.bcf` for biber, `\bibdata` in `.aux` for bibtex), so it works regardless of whether the bibliography is defined in a separate `.bib` file or embedded in the document.

If `el init` was run before any `.bib` files existed, `el compile` discovers them from those same auxiliary files and updates `.el.json` automatically.

After each successful compilation, `el compile` formats and validates every registered `.bib` file:

**Formatting** — entries are rewritten in a canonical style:
- Fields sorted in a standard order per entry type (e.g. `author`, `title`, `journal`, `year`, `volume`, `number`, `pages`, `doi` for `@article`)
- Values normalized to `{braced}` form
- Field names aligned with spaces for readability
- Trailing comma after the last field

The file is only rewritten if the content actually changes.

**Metadata validation** — each entry is checked against an external source the first time it is seen (results are cached in `.aux_dir/bib_cache.json` and not re-fetched on subsequent compiles):

- Entry has a `doi` field (or a `url` containing `doi.org`) → queried against the [Crossref API](https://api.crossref.org); mismatched fields are auto-corrected in place.
- Entry has an `eprint` field with `archiveprefix = {arXiv}` / `eprinttype = {arXiv}`, or a `url` pointing to `arxiv.org` → queried against the arXiv API; title, author, and year are auto-corrected if needed.
- Entry has neither → a one-time warning is printed and the entry is skipped.

Corrections are reported on the terminal:

```
[bib] Smith2023: corrected title, pages
[bib] Jones2021: no DOI or arXiv ID — skipping validation
```

Use `-o` / `--open` to open the PDF immediately after compilation:

```
$ el compile -o
```

## Installation

Requires Go 1.21+ and a working TeX Live / MacTeX installation.

```bash
git clone git@github.com:MarkAureli/easy-latex.git
cd easy-latex
make install
```

This places the `el` binary in `~/go/bin`. Make sure that directory is in your `$PATH`:

```zsh
# Add to ~/.zshrc
export PATH="$HOME/go/bin:$PATH"
```

### TeX tools not in PATH?

On macOS, TeX Live installs its binaries to `/Library/TeX/texbin/`. `el` checks this location automatically as a fallback for `pdflatex`, `bibtex`, and `biber`, so it works even if you haven't added it to your `$PATH`.

## What `el` adds to your project

Running `el init` and `el compile` in a LaTeX project creates the following:

| Path | Purpose |
|---|---|
| `.el.json` | Main `.tex` file, aux directory path, and registered `.bib` files |
| `.aux_dir/` | All pdflatex/bibtex/biber intermediate files, kept out of the project root |
| `.aux_dir/bib_cache.json` | Tracks which bib entries have already been validated |
| `<name>.pdf` | Symlink into `.aux_dir/`; open this in your PDF viewer |

In a git repository, `el init` registers `.aux_dir` and `.el.json` in `.git/info/exclude` automatically, so none of these files need to be added to `.gitignore`.
