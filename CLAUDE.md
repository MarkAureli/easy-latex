# easy-latex

CLI tool (`el`) for compiling LaTeX documents. Go project, module `github.com/MarkAureli/easy-latex`.

## Commands

- `el init` — detect main `.tex` file, create `.aux_dir/`, write `.el.json`, register git excludes. See `cmd/CLAUDE.md`.
- `el compile` — run pdflatex (+ bibtex/biber if needed), then format and validate all `.bib` files. See `cmd/CLAUDE.md`.

## Key files

| Path | Role |
|---|---|
| `.el.json` | Config: main tex file, aux dir, bib file paths |
| `.aux_dir/` | All pdflatex/bibtex/biber intermediates |
| `.aux_dir/bib_cache.json` | Per-entry validation source cache |
| `cmd/` | Cobra commands (`init`, `compile`) |
| `internal/bib/` | Bib parsing, key generation, formatting, validation |
| `internal/texscan/` | Tex file scanner for bib declarations |

## Bib processing (post-compile)

After each successful compile, every registered `.bib` file is:
1. Parsed and canonical keys assigned (`{LastName}{Year}{Title}`)
2. Each unseen entry validated via Crossref (if DOI present) or arXiv (if eprint present)
3. Fields normalised to the allowed set for the entry type; unknown fields dropped
4. Mandatory fields checked; warnings printed for missing ones
5. File rewritten only if content changed

See `internal/bib/CLAUDE.md` for entry-type specs.
