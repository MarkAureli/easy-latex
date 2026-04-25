# easy-latex

A minimal CLI tool for compiling LaTeX documents without the noise.

## Commands

### `el init`

Run in a folder containing a `.tex` file with `\begin{document}`. Detects the main file, creates an `.el/` working directory, and saves the configuration to `.el/config.json`.

```
$ el init
Initialized. Main file: thesis.tex
Bib files: bibliography.bib
```

If multiple eligible `.tex` files are found, you will be prompted to pick one.

`el init` scans the main file (and any files pulled in via `\input{}`/`\include{}`) for bibliography declarations — both `\bibliography{}` (bibtex) and `\addbibresource{}` (biblatex) — and consolidates all discovered `.bib` files into at most two files in the project root:

- `bibliography.bib` — all regular entries
- `preamble.bib` — `@string` and `@preamble` definitions (only created if non-empty)

Original `.bib` files are removed and all `\bibliography`/`\addbibresource` references in `.tex` files are rewritten to point to the new files.

Embedded bib content (`\begin{filecontents}{*.bib}...`) is extracted to disk before processing.

In a git repository, `el init` appends `.el` to `.git/info/exclude` automatically, so generated files are never accidentally committed.

Use `--ieee` to use IEEE-style bib file names (`IEEEabrv.bib` instead of `preamble.bib`) and enable IEEE formatting in the config.

### `el compile`

Compiles the document using `pdflatex`. Only warnings and errors are printed — all other LaTeX output is suppressed. On success, a copy of the PDF is placed in the project root.

```
$ el compile
Compiled successfully -> thesis.pdf
```

If the document uses a bibliography, `el` automatically detects the required tool and runs the full compilation sequence:

- `\bibliography{}` (natbib, plain bibtex) → `pdflatex` → `bibtex` → `pdflatex`
- `\usepackage{biblatex}` → `pdflatex` → `biber` → `pdflatex`

Detection is based on the auxiliary files produced by the first `pdflatex` pass (`.bcf` for biber, `\bibdata` in `.aux` for bibtex), so it works regardless of how the bibliography is set up.

If `el init` was run before any `.bib` files existed, `el compile` discovers them from those same auxiliary files and updates `.el/config.json` automatically.

Up to two additional pdflatex passes run automatically if needed (e.g. when LaTeX reports "rerun"), allowing up to four total passes to stabilize cross-references and citations.

Use `-o` / `--open` to open the PDF immediately after compilation:

```
$ el compile -o
```

#### Bib processing

The bib cache (`.el/bib.json`) is the source of truth for all bibliography entries. After each successful compilation, `el compile` writes `bibliography.bib` from the cache, including only cited entries with all configured transforms applied.

If `bibliography.bib` changed since the last compile, new entries are automatically parsed and added to the cache. When a new entry's canonical key differs from its original key, `\cite{}` references in all `.tex` files are rewritten automatically.

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

**Metadata validation** — each entry is checked against an external source the first time it is seen (results are cached in `.el/bib.json`). Entries that fail due to a transient error (timeout, rate limit, server error) are marked `source: "timeout"` and automatically retried on the next parse (disable with `el config set retry-timeout false`). Entries whose identifier is not found are marked `source: "invalid-id"` and not retried.

- Entry has a `doi` field (or a `url` containing `doi.org`) → queried against the [Crossref API](https://api.crossref.org); mismatched fields are auto-corrected in place and the entry type is set from Crossref's `type` field (e.g. `journal-article` → `@article`, `proceedings-article` → `@inproceedings`). For `@article` entries, the journal name is mechanically abbreviated to its ISO 4 form using the [LTWA](https://www.issn.org/services/online-services/access-to-the-ltwa/) (e.g. `Nature Communications` → `Nat. Commun.`). For proceedings and collection types, Crossref's `container-title` maps to `booktitle` instead of `journal`.
- Entry has an `eprint` field with `archiveprefix`/`eprinttype = {arXiv}`, or a `url` pointing to `arxiv.org` → queried against the arXiv API. If the arXiv response contains a DOI (i.e. the paper has been published), the entry is automatically redirected to Crossref validation with the correct entry type and full metadata. If Crossref is unavailable, it falls back to arXiv-only correction of title, author, and year.
- Entry has neither → a one-time warning is printed for types where `doi` is mandatory (`@article`, `@inproceedings`, `@conference`); silently skipped for all other types.

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

### `el config`

View or update processing options. Settings are stored in `.el/config.json` (local, per-project) and `~/.elconfig.json` (global). Local settings override global settings.

List the effective configuration with `list`. Use `list --global` to show only global settings (works outside projects):

```
$ el config list
SETTING                VALUE          SOURCE
abbreviate-journals    true           (default)
abbreviate-first-name  true           (default)
brace-titles           false          (default)
ieee-format            false          (default)
max-authors            0 (unlimited)  (default)
url-from-doi           false          (default)
retry-timeout          true           (default)
no-block-citations     false          (default)
no-math-linebreak      false          (default)
```

Set and unset values:

```
$ el config set ieee-format              # bool without value → true
$ el config set max-authors 3
$ el config unset max-authors            # non-bool: removes from config
$ el config unset brace-titles           # bool: sets to false
$ el config set --global ieee-format     # writes to ~/.elconfig.json
```

| Key | Type | Default | Effect |
|---|---|---|---|
| `abbreviate-journals` | bool | true | Abbreviate journal names to ISO 4 form |
| `abbreviate-first-name` | bool | true | Abbreviate first/middle names to initials |
| `brace-titles` | bool | false | Wrap title field in double braces `{{…}}` |
| `ieee-format` | bool | false | IEEE mode: forces brace titles, max 5 authors, converts arXiv `@misc` to `@unpublished` |
| `max-authors` | int | 0 | Truncate author list (0 = unlimited); IEEE implies 5 if unset |
| `url-from-doi` | bool | false | Replace `url` field with `https://doi.org/<doi>` when DOI is present |
| `retry-timeout` | bool | true | Re-validate entries that previously timed out during validation |

#### Pedantic checks

Each pedantic check is a top-level bool key. Enable to enforce style rules during compilation. Violations are errors — the PDF is still produced, but the command exits non-zero.

```
$ el config set no-block-citations          # enable a check
$ el config unset no-block-citations        # disable (sets to false)
```

| Check | What it flags |
|---|---|
| `no-block-citations` | Multi-key citations (`\cite{a,b}`) or adjacent cite commands (`\cite{a}\cite{b}`, `\cite{a}~\cite{b}`) |
| `no-math-linebreak` | Inline math (`$...$` or `\(...\)`) that spans multiple lines in the final PDF |

### `el bib`

Manage the bibliography cache.

#### `el bib add <ID>`

Add a single bibliography entry to the cache from a DOI or arXiv ID, without needing a `.bib` file. Shows progress during API calls and entry details on success.

```
$ el bib add 10.1038/s41586-023-06096-3
[bib] tmp: fetching metadata from Crossref...
Added "Smith2023AGreatPaper" to bib cache.
  Title:  A Great Paper
  Author: Smith, John et al.
  Source: crossref

$ el bib add 2301.07041
[bib] tmp: fetching metadata from arXiv...
Added "Doe2023SomePreprint" to bib cache.
  Title:  Some Preprint
  Author: Doe, Jane
  Source: arxiv
```

Accepts bare DOIs (`10.…`), `doi.org/` URLs, bare arXiv IDs (`2301.07041`, `2301.07041v2`), old-format arXiv IDs (`hep-th/0401234`), and `arxiv.org/abs/…` URLs. Duplicate entries (by DOI or arXiv ID) are detected and the existing key is returned. For arXiv IDs, if the paper has a published DOI, the entry is automatically upgraded to a full Crossref-validated `@article`.

Network requests retry automatically on transient failures (rate limits, server errors, timeouts) with exponential backoff.

The entry will appear in `bibliography.bib` on the next `el compile` when cited.

#### `el bib list`

Show all entries in the bib cache as a table.

```
$ el bib list
KEY                         TYPE      SOURCE    TITLE
Smith2023AGreatPaper        article   crossref  A Great Paper
Doe2023SomePreprint         misc      arxiv     Some Preprint

2 entries in bib cache.
```

### `el bib parse`

Pre-populate the bib cache from registered `.bib` files without compiling. Useful for validating entries against Crossref/arXiv ahead of time, or for re-populating the cache after deleting `.el/bib.json`. Shows progress during API calls and announces key renames.

```
$ el bib parse
[bib] key renamed: smith23 -> Smith2023AGreatPaper
Allocated 5 new bib cache entries.
```

### `el lsp`

Start a Language Server Protocol server over stdio that provides cite-key completions. Intended for editor integration (VS Code, Neovim, etc.).

Typing any cite command (`\cite{`, `\citet{`, `\citep{`, `\citealt{`, `\citeauthor{`, `\citeyear{`, etc.) triggers completion with all known cite keys from the bib cache. Capitalised (`\Citet{`), starred (`\citet*{`), and optional-argument (`\citep[see]{`) forms are also supported. Keys are loaded once at startup — restart the LSP to pick up new entries.

## Installation

Requires Go 1.26+ and a working TeX Live / MacTeX installation.

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

On macOS, TeX Live installs its binaries to `/Library/TeX/texbin/`; on Linux, to `/usr/local/texlive/<year>/bin/<arch>/`. `el` checks these locations automatically as a fallback for `pdflatex`, `bibtex`, and `biber`, so it works even if you haven't added them to your `$PATH`.

## Shell completion

`el` supports tab-completion for all commands and flags. Add one of the following to your shell config:

**Zsh** (`~/.zshrc`):

```zsh
source <(el completion zsh)
```

**Bash** (`~/.bashrc`):

```bash
source <(el completion bash)
```

**Fish** (`~/.config/fish/config.fish`):

```fish
el completion fish | source
```

Restart your shell or source the config file, then `el <TAB>` will complete commands and flags.

## What `el` adds to your project

Running `el init` and `el compile` in a LaTeX project creates the following:

| Path | Purpose |
|---|---|
| `.el/config.json` | Main `.tex` file, aux directory path, registered `.bib` files, and processing options |
| `.el/bib.json` | Bib cache: validated entry data (source of truth for bibliography generation) |
| `.el/` | All pdflatex/bibtex/biber intermediate files, kept out of the project root |
| `<name>.pdf` | Copy of compiled PDF from `.el/` |

In a git repository, `el init` registers `.el` in `.git/info/exclude` automatically, so none of these files need to be added to `.gitignore`.
