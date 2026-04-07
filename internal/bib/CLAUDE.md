# internal/bib

Handles all `.bib` file processing. Entry point: `ProcessBibFiles(bibFiles, auxDir)`.

## Pipeline (`validate.go`)

For each bib file:
1. Parse → `[]Item` (entries + raw chunks)
2. `assignCanonicalKeys` (first pass)
3. For each unseen entry: `validateEntry` → Crossref or arXiv correction; store pending cache entries
4. `normalizeEntryFields` — drop disallowed fields, resolve synonyms, derive url from doi
5. `warnMissingFields` — print `[bib] <key>: missing mandatory fields: ...`
6. `ensureArticleOptionalFields` — add blank `{}` for `volume`, `number`, `pages` on `@article`
7. `sortedFields` — reorder to canonical field order
8. `assignCanonicalKeys` (second pass — keys may have changed after Crossref correction)
9. Flush pending cache entries under final canonical keys
10. Rewrite file only if content changed

## Key generation (`key.go`)

Canonical key: `{LastName}{Year}{Title}` — CamelCase, LaTeX accents resolved to ASCII, math stripped.
`@unpublished` with no year: `{LastName}{Title}`.
Fallback to existing key if any required component is empty.
Disambiguate collisions with `a`, `b`, `c`, … suffix.

## Entry type specs

`entrySpecs` in `validate.go` and `canonicalOrder` in `format.go` must be kept in sync.

| Type | Mandatory | Allowed (= field order) |
|---|---|---|
| `@article` | author, year, title, journal, doi, url | + volume, number, pages |
| `@book` | author, year, title, publisher | + address, doi, url |
| `@incollection` | author, year, title, booktitle, publisher | + address, pages, doi, url |
| `@inproceedings` / `@conference` | author, year, title, booktitle, doi, url | + pages |
| `@phdthesis` / `@mastersthesis` | author, year, title, school, url | + doi |
| `@techreport` | author, year, title, institution, url | + doi |
| `@misc` (base) | author, year, title, url | + doi |
| `@misc` (arXiv) | author, year, title, eprint, archiveprefix | + primaryclass |
| `@unpublished` | author, title, note | + year, doi, url |

### Special rules

- `@article`: `issue` is a synonym for `number`; `volume`, `number`, `pages` always emitted (blank `{}` if absent)
- `@misc` arXiv detection: `eprint` + `archiveprefix`/`eprinttype = arXiv` (case-insensitive), or `url` matching `arxiv.org/abs/…`; `archiveprefix` always normalised to `{arXiv}`
- All types with `doi` + `url` allowed: `url` derived from `doi` as `https://doi.org/{doi}` if absent (not for arXiv misc)
- No-id warning (no DOI or arXiv ID) suppressed for types where `doi` is not mandatory

## Validation sources

- **Crossref**: entry has `doi` field or `url` containing `doi.org/`; corrects title, author, journal, year, volume, number, pages, doi
- **arXiv**: entry has qualifying `eprint` or `arxiv.org` url; corrects title, author, year
- Results cached in `.aux_dir/bib_cache.json` under the final canonical key

## Files

| File | Contents |
|---|---|
| `parse.go` | `ParseFile`, `Entry`, `Field`, `Item`, `FieldValue`, `SetField` |
| `key.go` | `GenerateKey`, `assignCanonicalKeys`, `latexToASCII`, accent maps |
| `format.go` | `canonicalOrder`, `renderItems`, `formatEntry`, `sortedFields` |
| `validate.go` | `ProcessBibFiles`, `entrySpecs`, normalization, validation, Crossref/arXiv queries |
