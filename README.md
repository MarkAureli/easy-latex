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

**Citation key normalisation** — each entry's key is rewritten to the canonical form `{LastName}{Year}{Title}`:

- `LastName` is the first author's last name in CamelCase (e.g. `VanDerBerg` for a compound name, `GarciaLopez` for a hyphenated one); for organisation authors the full name is used (e.g. `{Google Quantum AI}` → `GoogleQuantumAi`)
- `Year` is the four-digit publication year
- `Title` is the title in CamelCase with math mode (`$…$`) and LaTeX commands stripped; accents are resolved to ASCII (`\"u` → `ue`, `\'e` → `e`, `ß` → `ss`)
- For `@unpublished` entries `year` is optional; if absent the key is `{LastName}{Title}`
- If two entries produce the same canonical key a lowercase letter suffix disambiguates them (`Smith2023Fooa`, `Smith2023Foob`, …)

Example: an entry for "A Great Paper" by Smith in 2023 becomes `Smith2023AGreatPaper`.

**Formatting** — every known entry type is rewritten with a fixed field set in a fixed order; all other fields are dropped. Values are normalised to `{braced}` form, field names are space-aligned, and a trailing comma follows the last field.

| Type | Field order |
|---|---|
| `@article` | `author, year, title, journal, volume, number, pages, doi, url` |
| `@book` | `author, year, title, publisher, address, doi, url` |
| `@incollection` | `author, year, title, booktitle, publisher, address, pages, doi, url` |
| `@inproceedings` / `@conference` | `author, year, title, booktitle, pages, doi, url` |
| `@phdthesis` / `@mastersthesis` | `author, year, title, school, doi, url` |
| `@techreport` | `author, year, title, institution, doi, url` |
| `@misc` | `author, year, title, doi, url` — or for arXiv entries: `author, year, title, eprint, archiveprefix, primaryclass` |
| `@unpublished` | `author, year, title, doi, url, note` |

**Author formatting** — the `author` field is normalised uniformly across all entry types. Individual authors are written as `Last, F. M.` (last name followed by space-separated abbreviated initials); multiple authors are separated by ` and `:

```bibtex
author = {Smith, J. F. and Doe, J.},
```

Organisation names must be wrapped in an extra pair of braces to be treated as a single unit rather than a personal name:

```bibtex
author = {{Google Quantum AI}},
```

Additional rules:

- `@article`: `volume`, `number`, and `pages` are always emitted (blank `{}` if absent) for compatibility with bib styles that require them; `issue` is accepted as a synonym for `number`
- `@misc` arXiv detection: an entry with `eprint` + `archiveprefix`/`eprinttype = {arXiv}`, or a `url` pointing to `arxiv.org`, is treated as an arXiv entry; `archiveprefix` is always normalised to `{arXiv}`
- For all types that include `doi` and `url`: `url` is derived from `doi` as `https://doi.org/{doi}` if not otherwise present

The file is only rewritten if the content actually changes.

**Metadata validation** — each entry is checked against an external source the first time it is seen (results are cached in `.aux_dir/bib.json` and not re-fetched on subsequent compiles):

- Entry has a `doi` field (or a `url` containing `doi.org`) → queried against the [Crossref API](https://api.crossref.org); mismatched fields are auto-corrected in place. For `@article` entries, the journal name returned by Crossref is mechanically abbreviated to its ISO 4 form using the [LTWA](https://www.issn.org/services/online-services/access-to-the-ltwa/) (e.g. `Nature Communications` → `Nat. Commun.`).
- Entry has an `eprint` field with `archiveprefix`/`eprinttype = {arXiv}`, or a `url` pointing to `arxiv.org` → queried against the arXiv API; title, author, and year are auto-corrected if needed.
- Entry has neither → a one-time warning is printed for types where `doi` is mandatory (`@article`, `@inproceedings`, `@conference`, `@incollection`); silently skipped for all other types.

Mandatory fields per type (a warning is printed for any that remain empty after validation):

| Type | Mandatory fields |
|---|---|
| `@article` | `author, year, title, journal, doi, url` |
| `@book` | `author, year, title, publisher` |
| `@incollection` | `author, year, title, booktitle, publisher` |
| `@inproceedings` / `@conference` | `author, year, title, booktitle, doi, url` |
| `@phdthesis` / `@mastersthesis` | `author, year, title, school, url` |
| `@techreport` | `author, year, title, institution, url` |
| `@misc` (base) | `author, year, title, url` |
| `@misc` (arXiv) | `author, year, title, eprint, archiveprefix` |
| `@unpublished` | `author, title, note` |

Corrections are reported on the terminal:

```
[bib] Smith2023AGreatPaper: corrected title, pages
[bib] Jones2021Work: no DOI or arXiv ID — skipping validation
[bib] Brown2020Study: missing mandatory fields: doi, url
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
| `.aux_dir/bib.json` | Tracks which bib entries have already been validated |
| `<name>.pdf` | Symlink into `.aux_dir/`; open this in your PDF viewer |

In a git repository, `el init` registers `.aux_dir` and `.el.json` in `.git/info/exclude` automatically, so none of these files need to be added to `.gitignore`.
