# internal/bib

Handle `.bib` file processing.

## Architecture

Two-phase design: **cache allocation** (parse + validate) and **bib generation** (cache → file with config transforms). The validation cache is **global per user**, not per-project — see `global.go`.

- `AllocateCacheEntries(bibFiles, retryTimeout, log Logger)` (`validate.go`) — seeds the global bib cache from bib files (used by `el init`, `el bib parse`, auto-triggered by `el compile` when `bibliography.bib` hash changes). Wraps work in `withGlobalLock`.
- `WriteBibFromCache(path, citeKeys, opts)` (`validate.go`) — reconstructs entries from the global cache for cited keys, applies all config transforms via `WriteOptions` struct, writes `bibliography.bib` (used by `el compile` after pass 1). Cited keys present in the global cache but not yet in the project bib file are materialised at this step (auto-import on compile).
- Cache I/O in `cache.go`; global path resolution + lock in `global.go`/`cache.go`; Crossref HTTP in `crossref.go`; arXiv HTTP in `arxiv.go`; Zenodo HTTP in `zenodo.go`; DataCite HTTP in `datacite.go`
- `Version` constant in `validate.go` (used by Crossref User-Agent and CLI `--version`)

### Global bib cache (`global.go`, `cache.go`)

`GlobalBibPath() (string, error)` resolves the cache file path:

1. `$EL_GLOBAL_BIB` (escape hatch; used by tests via `t.Setenv`)
2. `$XDG_DATA_HOME/easy-latex/bib.json`
3. darwin: `~/Library/Application Support/easy-latex/bib.json`
4. other unix: `~/.local/share/easy-latex/bib.json`

`ensureGlobalDir()` creates the parent directory lazily. `saveCache` writes via `bib.json.tmp` + `os.Rename` for atomicity. `withGlobalLock(fn func() error) error` serialises read-modify-write across processes via `syscall.Flock(LOCK_EX)` on a sibling `bib.json.lock` file (a stable path so renames of the data file do not break the mutex). Read-only callers (`LoadCacheEntries`, `LoadCacheKeys`, lsp completion) skip the lock — atomic rename means readers either see the old file or the new file, never a torn write.

### Logger (`logger.go`)

`Logger` interface: `Info(msg)`, `Warn(msg)`, `Progress(msg)`. Implementations: `nopLogger` (silent), `stderrLogger` (plain stderr). Commands provide colored loggers (see `cmd/biblog.go`). `logOrNop(log)` returns `nopLogger` if nil. All bib-internal functions accept `Logger` parameter for user-facing messages.

### HTTP retry (`retry.go`)

`doWithRetry(req, log) (*http.Response, error)` — exponential backoff (1s/2s/4s), max 3 attempts. Retries on: HTTP 429, 5xx (`retryableStatusCode`), timeouts/connection errors (`isRetryableError`). `friendlyHTTPError(status, url)` converts codes to human-readable messages ("not found in Crossref" instead of "HTTP 404"). Used by `queryCrossref` and `queryArxiv`.

### HTTP client (`crossref.go`)

`httpClient` shared by Crossref and arXiv: 30s timeout, `throttledTransport` wrapping `http.DefaultTransport` enforces `minRequestGap = 300ms` between requests (≈3 req/s, under arXiv's 4 req/s burst limit). Both `queryCrossref` and `queryArxiv` set `User-Agent: easy-latex/<Version> (<repo>)`.

### arXiv batch prefetch (`arxiv.go`)

`PrefetchArxivIDs(ids []string, log Logger)` fetches metadata for up to `arxivBatchSize = 100` arXiv ids per request via the API's `id_list=a,b,c` parameter and stores parsed entries in an in-process `arxivPrefetch` map keyed by `arxivCanonical(id)` (lowercase, version-stripped). `queryArxiv` consults this map first via `lookupArxivPrefetch` and consumes the cached entry without an HTTP call; cache miss falls back to a per-id fetch. `AllocateCacheEntries` pre-scans all bib files for uncached arXiv ids and calls `PrefetchArxivIDs` before the per-entry validation loop; `cmd/bib add` does the same for its CLI arguments. Batch failures are silent — per-id fallback produces the user-facing error.

## Single-entry insertion from ID (`validate.go`)

Entry point: `AddEntryFromID(id string, log Logger) (key string, isNew bool, err error)`.

Adds one entry to the global bib cache from a raw DOI or arXiv identifier (no bib file needed). Returns `isNew=true` when entry was newly created, `false` when already cached (dedup hit). Wraps work in `withGlobalLock`.

- `normalizeDOI(s)` — strips `doi.org/` prefixes, returns bare DOI if starts with `10.`, else `""`
- `normalizeArxivID(s)` — matches `reArxivBare` (bare form) or `reArxivURL` (full URL), returns bare ID or `""`
- **Zenodo DOI path** (`zenodoRecordID(doi) != ""`): dedup by input DOI, call `queryZenodo`, dedup again by resolved concept DOI (`raw.Fields["doi"]`), then mandatory check (`"Zenodo"`), `source: "zenodo"`
- **arXiv DOI** (`arxivIDFromDOI(normalizeDOI(id)) != ""`): `id` replaced by the arXiv id → arXiv path
- **DataCite fallback**: Crossref `errNotFound` → `queryDataCite`; dedup by returned DOI; mandatory check (`"DataCite"`); `source: "datacite"`. Both 404 → error "DOI not found in Crossref or DataCite"
- **DOI path**: dedup via `cachedKeyByDOI` (case-insensitive); calls `queryCrossref`; entry type set from Crossref `type` field (defaults to `@article`)
- **arXiv path**: dedup via `Fields["eprint"]`; calls `queryArxiv`; if DOI returned, dedup by DOI then redirect to `queryCrossref` (becomes `@article`, `source: "crossref"`); on Crossref failure, falls back to `@misc` with `archiveprefix: {arXiv}`
- Key generated by `GenerateKey`; collision-disambiguated by `disambiguateKey` (appends `a`/`b`/… suffix)
- Returns `ErrUnrecognizedID` (sentinel `fmt.Errorf`) when format unrecognized — caller can check `err == bib.ErrUnrecognizedID`
- **Strict mandatory-field check**: after the API response is mapped onto the entry, `checkAPIMandatoryFields` normalises a copy (so `url`-from-`doi` is derived) and verifies every entry in `entrySpecs[type].mandatory` is non-empty. If anything is missing the call fails with `"Crossref/arXiv metadata missing mandatory fields for @<type>: <names>"` and the global cache is NOT written. Applies to all three commit points (Crossref-direct, arXiv→Crossref redirect, arXiv-only fallback). Catches Crossref records like `10.1007/b97696` where Springer never deposited authors

## Cache allocation (`validate.go`)

Entry point: `AllocateCacheEntries(bibFiles []string, retryTimeout bool, log Logger) (int, map[string]string, error)`.

Returns count of newly cached entries and a renames map (old key → new canonical key). Parses each bib file and seeds the global bib cache with any entries not yet cached. Parse-only: does NOT rewrite files or run full normalization pipeline. Used by `el init` and `el bib parse` to pre-populate cache without compile. Wraps work in `withGlobalLock`.

Deduplication:
- **Entries with DOI**: matched by DOI (case-insensitive). Crossref- (or Zenodo-/DataCite-) validated if new. **Dedup hit** (DOI or eprint already cached, possibly under another key — `el bib add`, other project): item mapped to the cached key via `cachedKeyByDOI`/`cachedKeyByEprint` → `aliasKey` → rename, so its `\cite{}`s get rewritten. Zenodo version DOI whose resolved concept DOI is already cached (`aliasKey`) or pending earlier in the same file (`aliasOwner`) is not added; its key is set to the owner's key (→ renames). Aliases are excluded from the second `assignCanonicalKeys` pass (run on a cloned `view`) so they never force an `a`/`b` suffix onto the owner.
- **Entries with arXiv ID**: matched by eprint field (case-insensitive). arXiv-validated if new.
- **No-ID entries**: matched by canonical cite key. Added as `source: "no-id"` placeholder if new.
- **Timeout entries**: when `retryTimeout=true`, excluded from dedup → re-validated on next run.

### Validation source tracking

Cache entries store a `Source` field indicating validation status:

| Source | Meaning | Re-validated? |
|---|---|---|
| `"crossref"` | Validated via Crossref | No |
| `"arxiv"` | Validated via arXiv | No |
| `"zenodo"` | Validated via Zenodo API | No |
| `"datacite"` | Validated via DataCite API (Crossref 404) | No |
| `"no-id"` | No DOI or arXiv ID present | No |
| `"invalid-id"` | DOI/arXiv ID not found (404, empty feed) | No |
| `"timeout"` | Transient failure (timeout, rate limit, server error) | When `retryTimeout=true` |

`errNotFound` sentinel (`retry.go`) distinguishes "not found" from transient errors. `validateEntry` uses `errors.Is(err, errNotFound)` to classify.

For new DOI/arXiv entries, runs `validateEntry` and stores full field snapshot. Does not apply config transforms (abbreviateJournals, braceTitles, etc.) — those are applied at compile time.

## Bib generation from cache (`validate.go`)

Entry point: `WriteBibFromCache(path string, citeKeys []string, opts WriteOptions) error`.

Reconstructs entries from the global cache for given cite keys and writes them to `path` (typically `bibliography.bib`) with all config transforms applied. Called by `el compile` after pass 1 to produce bib file from cache. Only cited keys written. Cited keys present in the global cache but missing from the project bib file are materialised here automatically (auto-import on compile).

## Renames persistence (`validate.go`)

- `LoadRenames(auxDir) map[string]string` — load `.el/renames.json` (old key → new key)
- `SaveRenames(auxDir, renames)` — persist renames map
- `ClearRenames(auxDir)` — delete `.el/renames.json`

Produced by `AllocateCacheEntries` when canonical key differs from bib file key. Consumed by `el compile` step 4 to rewrite `\cite{}` before bibliography generation.

## Bib hash tracking (`validate.go`)

- `BibFileChanged(bibPath, auxDir) bool` — true if `bibliography.bib` SHA256 differs from stored hash in `.el/bib_hash`
- `UpdateBibHash(bibPath, auxDir)` — update stored hash after write

## Unicode→LaTeX escaping (`format.go`)

`escapeUnicode(s)` replaces Unicode accented characters with LaTeX escape sequences at render time (inside `formatEntry`). Applied to all field values alongside `escapeAmpersand` and `escapeUnderscore`. Map covers: acute, grave, circumflex, tilde, umlaut, macron, dot, caron, breve, ring, double acute, cedilla, ogonek, and standalone commands (ß, æ, œ, å, ø, ł, ð, þ, ŋ). Cache stores Unicode; escaping happens only on output.

`escapeUnderscore(s)` replaces unescaped `_` with `\_` in bib field values. Applied only on `.bib` output (`formatEntry`); cache (`bib.json`) stores raw values.

`escapePercent(s)` replaces unescaped `%` with `\%` in bib field values so BibTeX does not treat rest of line as comment. Applied only on `.bib` output; cache stores raw values.

`escapeHash(s)` replaces unescaped `#` with `\#` so LaTeX does not treat it as a macro parameter delimiter.

`escapeTilde(s)` replaces unescaped `~` with `\textasciitilde{}` so LaTeX does not treat it as a non-breaking space.

`unescapeFieldValue(s)` reverses all the above escaping — used by `allocateBibFile` when reading field values from `.bib` files into the cache so raw values are stored (escaping only on output).

## Key generation (`key.go`)

Canonical key: `{LastName}{Year}{Title}` — CamelCase, LaTeX accents→ASCII, math stripped.
`@unpublished` with no year: `{LastName}{Title}`.
Fallback to existing key if component missing.
Disambiguate collisions with `a`, `b`, `c`, … suffix.

## Entry type specs

Keep `entrySpecs` in `validate.go` and `canonicalOrder` in `format.go` in sync.

| Type | Mandatory | Allowed (= field order) |
|---|---|---|
| `@article` | author, year, title, journal, doi, url | + volume, number, pages |
| `@book` | author, year, title, publisher | + address, doi, url |
| `@incollection` | author, year, title, booktitle, publisher | + address, pages, doi, url |
| `@inproceedings` / `@conference` | author, year, title, booktitle, doi, url | + pages |
| `@phdthesis` / `@mastersthesis` | author, year, title, school, url | + doi |
| `@techreport` | author, year, title, institution, url | + doi |
| `@misc` (base) | author, year, title, url | + publisher, doi |
| `@misc` (arXiv) | author, year, title, eprint, archiveprefix | + primaryclass |
| `@unpublished` | author, title, note | + year, doi, url |

### Special rules

- `@article`: `issue` = synonym for `number`; `volume`, `number`, `pages` always emitted (blank `{}` if absent)
- `@misc` arXiv detection: `eprint` + `archiveprefix`/`eprinttype = arXiv` (case-insensitive), or `url` matching `arxiv.org/abs/…`; `archiveprefix` always normalised to `{arXiv}`
- All types with `doi` + `url` allowed: `url` derived from `doi` as `https://doi.org/{doi}` if absent (not for arXiv misc); when `urlFromDOI=true`, replaces existing url too
- No-id warn suppressed for types where `doi` not mandatory

## Validation sources

- **Crossref**: all fetched string fields run through `html.UnescapeString` to decode HTML entities (`&amp;`, `&lt;`, `&#8212;`, etc.) Crossref embeds in JSON; corrects title, author, journal/booktitle (full name), year, volume, number, pages (falls back to `article-number` when `page` absent, e.g. APS/IOP; degenerate `X-X` collapsed to `X` via `collapsePageRange`), doi, publisher (only for `@book`/`@incollection`); title cleaned via `cleanCrossrefTitle` (converts MathML to `$...$` and Crossref face markup `<i>/<sub>/<sup>/...` to LaTeX commands, then collapses whitespace runs incl. embedded newlines); author names normalised via `normalizeAllCapsName` (collapses Unicode whitespace runs to one ASCII space; all-caps → title case, any non-letter non-mark starts a new word so `I.M.` stays intact); **group/consortium authors** (Crossref `name` field without `given`/`family`) wrapped in braces `{Name}` so BibTeX treats them as single corporate entity; when any group author is present, all individual authors are omitted (only group authors kept); maps `type` to BibTeX entry type (`journal-article`→`@article`, `proceedings-article`→`@inproceedings`, `book-chapter`→`@incollection`, `book`/`monograph`/`edited-book`/`reference-book`→`@book`, `report`/`report-component`→`@techreport`, `dissertation`→`@phdthesis`; unknown types leave entry type unchanged); `container-title` maps to `journal` or `booktitle` based on mapped type; `@article` journal → `AbbreviateISO4` on live entry when `abbreviateJournals=true`
- **Placeholder DOIs**: `isPlaceholderDOI` = prefix `10.5555/` (Crossref test prefix; ACM DL pseudo-DOIs). `findDOI` ignores them (doi field and doi.org url) → entry follows arXiv/no-id path. `normalizeEntryFields` drops placeholder `doi` and a `url` matching `isPlaceholderDOIURL`. `AddEntryFromID` returns a "placeholder DOI" error
- **Zenodo** (`zenodo.go`): DOIs matching `(?i)^10\.5281/zenodo\.(\d+)$` bypass Crossref. `GET https://zenodo.org/api/records/{recid}` with `Accept: application/vnd.inveniordm.v1+json` (native InvenioRDM format; legacy JSON lacks `publisher` and imprint title). Concept recid → 302 to latest version (client follows). 410 Gone treated as 404. Stored DOI = `parent.pids.doi` (concept DOI) if present, else record DOI; version DOI input logs Info. No `version` field stored. Authors: `person_or_org.name` unchanged, joined by ` and `; `type == "organizational"` → `{Name}`. Year = first 4 chars of `publication_date`. Type: `zenodoCandidateType(resource_type.id)` — `publication-article`→article (`journal:journal` title/volume/issue→number/pages), `publication-conferencepaper`→inproceedings (booktitle = `imprint:imprint.title` else `meeting:meeting.title`, pages), `publication-section`→incollection (booktitle = imprint title, publisher, address = imprint place, pages), `publication-book`→book (publisher, address), `publication-dissertation`→phdthesis, or mastersthesis if `thesis:thesis.type` contains "master" (school = university); everything else misc (+ publisher). `zenodoEntry` probes the candidate type with Zenodo-only fields via `normalizeEntryFields` + `missingMandatoryFields`; any missing → misc. Raw fields only include fields allowed for the final type
- **arXiv DOIs**: `reArxivDOI` = `(?i)^10\.48550/arxiv\.(.+)$`; `findDOI` ignores them, `findArxivID` returns the id from `doi` field or doi.org `url` (checked after eprint/arxiv url). In `validateEntry` arXiv branch, entries identified via arXiv DOI (`entryRawDOI`) are forced to `@misc` + `eprint` + `archiveprefix = {arXiv}` when arXiv-only metadata is used (`asArxivMisc`); normalisation then drops doi/url (not in `arxivAllowed`)
- **DataCite** (`datacite.go`): fallback when Crossref returns `errNotFound` (validateEntry + AddEntryFromID). `GET https://api.datacite.org/dois/{PathEscape(doi)}`. Always `@misc`; fields author, year (`publicationYear`, string or number), title (first title without `titleType`, via `cleanCrossrefTitle`), publisher, doi. `formatDataciteCreators`: `nameType == "Organizational"` → `{name}`; family+given → `Family, Given` (`normalizeAllCapsName`); family only → family; else `name` unchanged. `isDataciteUnknown` drops `:xxx`/`(:xxx)` codes and `Unpublished`/`unknown`/`n/a`/… for creators and publisher. DataCite 404 → `invalid-id`; other errors → `timeout`
- **arXiv**: corrects title, author, year; author names normalised from all-caps to title case via `normalizeAllCapsName`; if `<arxiv:doi>` present in API response, redirects to Crossref validation (entry becomes `@article`, `source: "crossref"`); falls back to arXiv-only on Crossref failure
- Cache in `.el/bib.json` under final canonical key; stores entry `Type`, all allowed fields as snapshot (pre-config-transform: full journal, raw authors, pre-brace title, raw url from bib file)
- On subsequent compiles, all cached fields re-applied to entry before pipeline; journal re-abbreviated if `abbreviateJournals=true`; old-format crossref/arxiv entries (nil Fields) evicted and re-validated

## ISO 4 abbreviation (`iso4.go`)

`AbbreviateISO4(title string) string` — abbrev journal title per ISO 4.

- LTWA data in `ltwa.tsv` (37,670 entries, non-empty only); parse once via `sync.Once`
- Lookup: exact first, then longest-prefix (LTWA entries ending `-` = prefix patterns)
- Language priority when multiple entries match: `"multiple languages"` > `"english"` > first record
- First letter of abbrev mirrors original case
- Stop words (articles, prepositions, conjunctions, EN/FR/DE/IT/ES/LA) omitted; exception: `isSectionDesignator` keeps a single uppercase letter that is not the first word and not followed by a letter-initial word (`Physical Review A` → `Phys. Rev. A`, `Series E`)
- Single-word titles returned unchanged
- Trailing parentheticals stripped before processing: `"Lancet (London)"` → `"Lancet"`
- Hyphenated tokens split on `-`, each segment processed, rejoined

## Files

| File | Contents |
|---|---|
| `parse.go` | `ParseFile`, `Entry`, `Field`, `Item`, `FieldValue`, `SetField` |
| `key.go` | `GenerateKey`, `assignCanonicalKeys`, `latexToASCII`, accent maps |
| `format.go` | `canonicalOrder`, `RenderEntries`, `renderItems`, `formatEntry`, `sortedFields`, `stripNonEscapedBraces`, `escapeAmpersand`, `escapeHash`, `escapePercent`, `escapeTilde`, `escapeUnicode`, `escapeUnderscore`, `unescapeFieldValue`, `unicodeToLaTeX` |
| `author.go` | `formatAuthorField`, `formatSingleAuthor`, `abbreviateGivenNames`, `normalizeAllCapsName`, `initialOf`, `splitByAnd` |
| `validate.go` | `Version`, `WriteOptions`, `AllocateCacheEntries`, `WriteBibFromCache`, `AddEntryFromID`, `entrySpecs`, normalization, validation |
| `logger.go` | `Logger` interface, `nopLogger`, `logOrNop`, `stderrLogger` |
| `retry.go` | `doWithRetry`, `friendlyHTTPError`, `retryableStatusCode`, `isRetryableError` |
| `global.go` | `GlobalBibPath`, `ensureGlobalDir` |
| `cache.go` | `loadCache`, `loadCacheStrict`, `saveCache` (atomic tmp+rename), `withGlobalLock` (flock on `bib.json.lock`), `LoadRenames`, `SaveRenames`, `ClearRenames`, `BibFileChanged`, `UpdateBibHash`, `LoadCacheKeys`, `LoadCacheEntries`, `RemoveEntryFromCache`, `RemoveEntriesFromCache`, `CacheEntryInfo` |
| `crossref.go` | `httpClient`, `queryCrossref`, `mapCrossrefType`, `containerTitleField`, `formatCrossrefAuthors` |
| `arxiv.go` | `queryArxiv`, `formatArxivAuthors`, `reverseArxivName` |
| `datacite.go` | `reArxivDOI`, `arxivIDFromDOI`, `dataciteResponse`, `formatDataciteCreators`, `isDataciteUnknown`, `dataciteString`, `queryDataCite` |
| `zenodo.go` | `reZenodoDOI`, `zenodoRecordID`, `zenodoRecord`, `zenodoCandidateType`, `formatZenodoCreators`, `zenodoFields`, `zenodoEntry`, `queryZenodo` |
| `xmltitle.go` | `cleanCrossrefTitle`, MathML→LaTeX converter (`encoding/xml` Decoder), Crossref face markup→LaTeX, XML tag stripper |
| `iso4.go` | `AbbreviateISO4`, LTWA loader, prefix matcher, stop word list |
| `ltwa.tsv` | Embedded LTWA data (tab-separated: WORD, ABBREVIATION, LANGUAGES) |
