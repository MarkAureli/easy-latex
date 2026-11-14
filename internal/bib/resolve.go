package bib

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// resolved is the metadata fetched for one entry.
type resolved struct {
	entry  Entry      // input entry with API corrections applied
	raw    cacheEntry // raw API fields, cached before any config transform
	source string     // "zenodo", "crossref", "datacite" or "arxiv"
	// linked is the identifier key of an already-known entry the metadata
	// belongs to; set instead of looking that identifier up.
	linked string
}

// ids returns the index keys of the identifiers carried by the fetched
// metadata (e.g. a Zenodo concept DOI, or the DOI of a published preprint).
func (r resolved) ids() []string {
	ids := fieldIDs(r.raw.Fields)
	if r.linked != "" {
		ids = append(ids, r.linked)
	}
	return ids
}

// sourceLabel names each metadata source for user-facing messages.
var sourceLabel = map[string]string{
	"zenodo":   "Zenodo",
	"crossref": "Crossref",
	"datacite": "DataCite",
	"arxiv":    "arXiv",
}

var errNoID = errors.New("no DOI or arXiv ID")

// resolveEntry fetches metadata for e from the service its identifier
// belongs to:
//   - Zenodo DOI (10.5281/zenodo.*) → Zenodo
//   - other DOI → Crossref, falling back to DataCite for DOIs Crossref does
//     not know
//   - arXiv ID (eprint, arxiv.org url, 10.48550/arXiv.* DOI) → arXiv, then
//     Crossref if arXiv reports a journal DOI; the arXiv result is kept when
//     that Crossref lookup fails; if known reports that DOI as already
//     cached, the lookup is skipped and the result is linked to it
//
// known may be nil. Errors wrap errNotFound when the identifier is unknown
// (see failureSource).
func resolveEntry(e Entry, known func(id string) bool, log Logger) (resolved, error) {
	log = logOrNop(log)
	if doi := findDOI(e); doi != "" {
		return resolveDOI(e, doi, log)
	}
	if id := findArxivID(e); id != "" {
		return resolveArxiv(e, id, known, log)
	}
	return resolved{}, errNoID
}

func resolveDOI(e Entry, doi string, log Logger) (resolved, error) {
	if recid := zenodoRecordID(doi); recid != "" {
		r, raw, _, err := queryZenodo(e, recid, log)
		if err != nil {
			return resolved{}, fmt.Errorf("Zenodo query failed: %w", err)
		}
		return resolvedFrom(r, e, raw, "zenodo"), nil
	}
	r, raw, typ, err := queryCrossref(e, doi, log)
	if errors.Is(err, errNotFound) {
		r, raw, err := queryDataCite(e, doi, log)
		if errors.Is(err, errNotFound) {
			return resolved{}, fmt.Errorf("DOI not found in Crossref or DataCite: %w", errNotFound)
		}
		if err != nil {
			return resolved{}, fmt.Errorf("DataCite query failed: %w", err)
		}
		return resolvedFrom(r, e, raw, "datacite"), nil
	}
	if err != nil {
		return resolved{}, fmt.Errorf("Crossref query failed: %w", err)
	}
	res := resolvedFrom(r, e, raw, "crossref")
	res.entry.Type = cmp.Or(typ, res.entry.Type)
	return res, nil
}

func resolveArxiv(e Entry, id string, known func(string) bool, log Logger) (resolved, error) {
	r, raw, doi, err := queryArxiv(e, id, log)
	if err != nil {
		return resolved{}, fmt.Errorf("arXiv query failed: %w", err)
	}
	if doi != "" && known != nil && known(doiID(doi)) {
		res := resolvedFrom(r, e, raw, "arxiv")
		res.linked = doiID(doi)
		return res, nil
	}
	if doi != "" {
		log.Info(e.Key, fmt.Sprintf("arXiv entry has DOI %s, using Crossref", doi))
		cr, crRaw, typ, err := queryCrossref(e, doi, log)
		if err == nil {
			res := resolvedFrom(cr, e, crRaw, "crossref")
			res.entry.Type = cmp.Or(typ, "article")
			return res, nil
		}
		log.Warn(e.Key, fmt.Sprintf("Crossref query failed (%v), falling back to arXiv", err))
	}
	res := resolvedFrom(r, e, raw, "arxiv")
	if arxivIDFromDOI(entryRawDOI(e)) != "" {
		// Identified only by an arXiv DOI (10.48550/arXiv.<id>): keep it as an
		// arXiv @misc entry.
		res.entry.Type = "misc"
		SetField(&res.entry, "eprint", "{"+id+"}")
		SetField(&res.entry, "archiveprefix", "{arXiv}")
	}
	return res, nil
}

// resolvedFrom builds a resolved from a query result; queries return a nil
// entry when they changed nothing, in which case the input entry is used.
func resolvedFrom(corrected *Entry, input Entry, raw cacheEntry, source string) resolved {
	if corrected != nil {
		input = *corrected
	}
	return resolved{entry: input, raw: raw, source: source}
}

// failureSource is the cache source recorded for an entry whose lookup failed.
func failureSource(err error) string {
	if errors.Is(err, errNotFound) {
		return "invalid-id"
	}
	return "timeout"
}

// idIndex maps identifier keys (see doiID, eprintID) to the cache key of the
// entry holding that identifier.
type idIndex map[string]string

// newIDIndex indexes the DOIs and eprints of all entries in c, leaving out
// "timeout" entries when skipTimeouts is set.
func newIDIndex(c cache, skipTimeouts bool) idIndex {
	idx := idIndex{}
	for key, entry := range c {
		if skipTimeouts && entry.Source == "timeout" {
			continue
		}
		for _, id := range append(fieldIDs(entry.Fields), entry.Aliases...) {
			idx[id] = key
		}
	}
	return idx
}

func doiID(doi string) string {
	if doi == "" {
		return ""
	}
	return "doi:" + strings.ToLower(doi)
}

// eprintID is version-less, so 2301.00001v2 and 2301.00001 share a key.
func eprintID(id string) string {
	if id == "" {
		return ""
	}
	return "eprint:" + arxivCanonical(id)
}

// fieldIDs returns the identifier keys of a cache entry's fields.
func fieldIDs(fields map[string]string) []string {
	var ids []string
	for _, id := range []string{doiID(fields["doi"]), eprintID(fields["eprint"])} {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// addAliases records those of ids that e's fields and aliases do not carry
// yet, and reports whether e changed.
func (e *cacheEntry) addAliases(ids ...string) bool {
	have := map[string]bool{}
	for _, id := range append(fieldIDs(e.Fields), e.Aliases...) {
		have[id] = true
	}
	changed := false
	for _, id := range ids {
		if id != "" && !have[id] {
			e.Aliases = append(e.Aliases, id)
			have[id] = true
			changed = true
		}
	}
	if changed {
		slices.Sort(e.Aliases)
	}
	return changed
}

// entryID returns the identifier key resolveEntry looks e up by, or "" if e
// has no usable DOI or arXiv ID.
func entryID(e Entry) string {
	if doi := findDOI(e); doi != "" {
		return doiID(doi)
	}
	return eprintID(findArxivID(e))
}

// entryFromID builds a stub entry carrying the bare DOI or arXiv ID in id
// (URL forms accepted), ready for resolveEntry.
func entryFromID(id string) (Entry, error) {
	doi := normalizeDOI(id)
	if axID := arxivIDFromDOI(doi); axID != "" {
		id, doi = axID, ""
	}
	if doi != "" {
		if isPlaceholderDOI(doi) {
			return Entry{}, fmt.Errorf("%s is a placeholder DOI (Crossref test prefix 10.5555, used by the ACM Digital Library for items without a registered DOI)", doi)
		}
		return Entry{Type: "article", Key: "tmp", Fields: []Field{{Name: "doi", Value: doi}}}, nil
	}
	if axID := normalizeArxivID(id); axID != "" {
		return Entry{Type: "misc", Key: "tmp", Fields: []Field{
			{Name: "eprint", Value: axID},
			{Name: "archiveprefix", Value: "{arXiv}"},
		}}, nil
	}
	return Entry{}, ErrUnrecognizedID
}
