# el bibentry (`bibentry.go`)

Add a single entry to bib cache from a bare ID (no bib file needed). No config load required.

- **DOI** — bare (`10.1234/foo`), `doi.org/` prefix, or `https://doi.org/` prefix → query Crossref, cache as `@article`, `source: "crossref"`
- **arXiv ID** — new format `NNNN.NNNNN[vN]`, old format `cat/NNNNNNN`, or full `arxiv.org/abs/…` URL → query arXiv API, cache as `@misc`, `source: "arxiv"`
- **Unrecognized** — emits `[bib] warning: …` to stderr, exits 0 (no entry created)
- **Already cached** — dedup by `Fields["doi"]` or `Fields["eprint"]`; returns existing key with "Added" message

Calls `bib.AddEntryFromID(id, auxDir)`. Returns `bib.ErrUnrecognizedID` for unknown formats; other errors propagate.
