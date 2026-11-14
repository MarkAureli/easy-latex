package bib

import (
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
)

// Version is the application version, used in User-Agent headers.
const Version = "0.1.0"

// AllocateCacheEntries parses each bib file and seeds the bib cache with any
// entries not yet present, without rewriting the bib files.
//
// Duplicate detection:
//   - entries with a DOI  → deduplicated by DOI  (Crossref validated)
//   - entries with arXiv ID → deduplicated by arXiv ID (arXiv validated)
//   - no-ID entries       → deduplicated by canonical cite key
//
// Returns the number of newly added cache entries and a map of old→new key
// renames for any entries whose canonical key differs from their original key.
func AllocateCacheEntries(bibFiles []string, retryTimeout bool, log Logger) (added int, renames map[string]string, err error) {
	log = logOrNop(log)
	if len(bibFiles) == 0 {
		return 0, map[string]string{}, nil
	}
	err = withGlobalLock(func() error {
		added, renames, err = allocateCacheEntriesLocked(bibFiles, retryTimeout, log)
		return err
	})
	return added, renames, err
}

func allocateCacheEntriesLocked(bibFiles []string, retryTimeout bool, log Logger) (int, map[string]string, error) {
	c, err := loadCacheStrict()
	if err != nil {
		return 0, nil, err
	}

	// When retryTimeout is true, "timeout" entries stay out of the index so
	// they get re-validated.
	idx := newIDIndex(c, retryTimeout)

	// Pre-pass: collect all uncached arXiv ids across all bib files and
	// fetch them in batched API calls. Per-entry validation below then
	// consumes the prefetched results without further HTTP calls.
	var pendingArxiv []string
	for _, path := range bibFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, item := range ParseFile(string(data)) {
			if !item.IsEntry {
				continue
			}
			if findDOI(item.Entry) != "" {
				continue
			}
			id := findArxivID(item.Entry)
			if _, cached := idx[eprintID(id)]; id == "" || cached {
				continue
			}
			pendingArxiv = append(pendingArxiv, id)
		}
	}
	PrefetchArxivIDs(pendingArxiv, log)

	added := 0
	dirty := false
	allRenames := map[string]string{}
	for _, path := range bibFiles {
		n, aliased, renames, err := allocateBibFile(path, c, idx, log)
		if err != nil {
			return added, allRenames, err
		}
		added += n
		dirty = dirty || aliased
		maps.Copy(allRenames, renames)
	}
	if added > 0 || dirty {
		saveCache(c, log)
	}
	return added, allRenames, nil
}

// allocateBibFile seeds c from one bib file. It returns the number of added
// entries, whether aliases were added to already-cached entries, and renames.
func allocateBibFile(path string, c cache, idx idIndex, log Logger) (int, bool, map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false, nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	items := ParseFile(string(data))

	// Record original keys before canonical assignment so we can detect renames.
	originalKeys := make(map[int]string, len(items))
	for i, item := range items {
		if item.IsEntry {
			originalKeys[i] = item.Entry.Key
		}
	}

	assignCanonicalKeys(items)

	type pendingEntry struct {
		itemIdx int
		entry   cacheEntry
	}
	var pending []pendingEntry
	added := 0
	// An item whose identifier is already cached (possibly under another key,
	// e.g. from el bib add or another project) or claimed by an earlier item of
	// this file is merged into that entry instead of being added: aliasKey
	// maps it to a cache key, aliasOwner to the owning item. Identifiers the
	// target's fields lack are recorded as its aliases.
	aliased := false
	pendingIDs := map[string]int{}
	aliasKey := map[int]string{}
	aliasOwner := map[int]int{}
	alias := func(i int, ids []string) bool {
		for _, id := range ids {
			if key, ok := idx[id]; ok {
				aliasKey[i] = key
				for _, id := range ids {
					idx[id] = key
				}
				if entry, ok := c[key]; ok && entry.addAliases(ids...) {
					c[key] = entry
					aliased = true
				}
				return true
			}
			if owner, ok := pendingIDs[id]; ok {
				aliasOwner[i] = owner
				for _, id := range ids {
					pendingIDs[id] = owner
				}
				return true
			}
		}
		return false
	}
	known := func(id string) bool {
		_, cached := idx[id]
		_, claimed := pendingIDs[id]
		return cached || claimed
	}

	for i, item := range items {
		if !item.IsEntry {
			continue
		}
		e := item.Entry
		for j, f := range e.Fields {
			e.Fields[j].Value = unescapeFieldValue(f.Value)
		}
		rawURL := FieldValue(e, "url")
		id := entryID(e)

		if id == "" {
			if _, seen := c[e.Key]; seen {
				continue
			}
			normalizeEntryFields(&e, false)
			if title := FieldValue(e, "title"); title != "" {
				if normalized := stripNonEscapedBraces(title); normalized != title {
					SetField(&e, "title", "{"+normalized+"}")
				}
			}
			if warn := warnMissingFields(e); warn != "" {
				log.Warn(e.Key, warn)
			}
			items[i].Entry = e
			pending = append(pending, pendingEntry{i, buildCacheEntry(e, cacheEntry{}, "no-id", rawURL)})
			added++
			continue
		}

		if alias(i, []string{id}) {
			continue
		}
		res, err := resolveEntry(e, known, log)
		if err != nil {
			log.Warn(e.Key, err.Error())
			res = resolved{entry: e, source: failureSource(err)}
		}
		// The fetched metadata may carry another identifier (Zenodo concept
		// DOI, journal DOI of a preprint) that is already known.
		ids := append(res.ids(), id)
		if alias(i, ids) {
			continue
		}
		for _, id := range ids {
			pendingIDs[id] = i
		}
		items[i].Entry = res.entry
		pending = append(pending, pendingEntry{i, buildCacheEntry(res.entry, res.raw, res.source, rawURL)})
		added++
	}

	// Re-assign canonical keys: Crossref/arXiv/Zenodo may have corrected
	// author/title/year. Merged aliases are excluded so they cannot force a
	// collision suffix onto their owner, then take the owner's key.
	view := slices.Clone(items)
	for idx := range aliasOwner {
		view[idx].IsEntry = false
	}
	for idx := range aliasKey {
		view[idx].IsEntry = false
	}
	assignCanonicalKeys(view)
	for i := range items {
		if view[i].IsEntry {
			items[i].Entry.Key = view[i].Entry.Key
		}
	}
	for idx, owner := range aliasOwner {
		items[idx].Entry.Key = items[owner].Entry.Key
	}
	for idx, key := range aliasKey {
		items[idx].Entry.Key = key
	}
	for _, p := range pending {
		c[items[p.itemIdx].Entry.Key] = p.entry
	}
	// Later bib files see this file's identifiers under their final keys.
	for id, owner := range pendingIDs {
		key := items[owner].Entry.Key
		idx[id] = key
		if entry, ok := c[key]; ok && entry.addAliases(id) {
			c[key] = entry
		}
	}

	// Collect renames: entries whose canonical key differs from the original key.
	renames := map[string]string{}
	for i, item := range items {
		if !item.IsEntry {
			continue
		}
		if orig, ok := originalKeys[i]; ok && orig != item.Entry.Key {
			renames[orig] = item.Entry.Key
		}
	}
	return added, aliased, renames, nil
}

// buildCacheEntry snapshots a validated entry for the cache: its normalized
// fields overlaid with the raw API fields (empty for unvalidated entries).
func buildCacheEntry(e Entry, raw cacheEntry, source, rawURL string) cacheEntry {
	entry := cacheEntry{Source: source, Type: e.Type}
	eCopy := e
	normalizeEntryFields(&eCopy, false)
	fields := make(map[string]string, len(eCopy.Fields))
	for _, f := range eCopy.Fields {
		if v := FieldValue(eCopy, f.Name); v != "" {
			fields[f.Name] = v
		}
	}
	for k, v := range raw.Fields {
		if v != "" {
			fields[k] = v
		}
	}
	delete(fields, "url")
	if rawURL != "" {
		fields["url"] = rawURL
	}
	entry.Fields = fields
	return entry
}

// WriteOptions holds configuration for WriteBibFromCache.
type WriteOptions struct {
	AbbreviateJournals  bool
	BraceTitles         bool
	ArxivAsUnpublished  bool
	MaxAuthors          int
	AbbreviateFirstName bool
	UrlFromDOI          bool
}

// WriteBibFromCache generates path from the bib cache for the given cited keys,
// applying all config transforms. Returns an error if any key is absent from
// the cache (caller should run 'el bib parse' first).
func WriteBibFromCache(path string, citeKeys []string, opts WriteOptions) error {
	if len(citeKeys) == 0 {
		return nil
	}
	c := loadCache()

	items := make([]Item, 0, len(citeKeys))
	for _, key := range citeKeys {
		cached, ok := c[key]
		if !ok {
			return fmt.Errorf("cite key %q not found in bib cache; run 'el bib parse'", key)
		}

		e := Entry{Key: key, Type: cached.Type}
		for _, name := range slices.Sorted(maps.Keys(cached.Fields)) {
			val := cached.Fields[name]
			if name == "journal" && opts.AbbreviateJournals && cached.Source == "crossref" {
				SetField(&e, name, "{"+AbbreviateISO4(val)+"}")
			} else {
				SetField(&e, name, "{"+val+"}")
			}
		}

		normalizeEntryFields(&e, opts.UrlFromDOI)

		if opts.ArxivAsUnpublished && e.Type == "misc" && findArxivID(e) != "" {
			transformArxivMiscToUnpublished(&e)
		}

		if author := FieldValue(e, "author"); author != "" {
			SetField(&e, "author", "{"+formatAuthorField(author, opts.MaxAuthors, opts.AbbreviateFirstName)+"}")
		}

		if title := FieldValue(e, "title"); title != "" {
			if normalized := stripNonEscapedBraces(title); normalized != title {
				SetField(&e, "title", "{"+normalized+"}")
			}
		}

		if opts.BraceTitles {
			if title := FieldValue(e, "title"); title != "" {
				SetField(&e, "title", "{{"+title+"}}")
			}
		}

		ensureArticleOptionalFields(&e)
		e.Fields = sortedFields(e.Type, e.Fields)

		items = append(items, Item{IsEntry: true, Entry: e})
	}

	return os.WriteFile(path, []byte(renderItems(items)), 0644)
}

// ── entry specifications ───────────────────────────────────────────────────────

// typeSpec describes the allowed and mandatory fields for a bib entry type.
type typeSpec struct {
	// mandatory lists field names that must be non-empty.
	// A token of the form "a|b" means at least one of a or b must be present.
	mandatory []string
	// allowed is the complete set of fields kept during normalisation.
	// Fields absent from this set are dropped.
	allowed map[string]bool
	// synonyms maps canonical field name -> accepted alias.
	// When the canonical field is absent and the alias is present, the alias is renamed.
	synonyms map[string]string
	// arxivMandatory and arxivAllowed override mandatory/allowed when the entry
	// contains an arXiv identifier. Only used by @misc.
	arxivMandatory []string
	arxivAllowed   map[string]bool
}

var entrySpecs = map[string]typeSpec{
	"article": {
		mandatory: []string{"author", "title", "journal", "year", "doi", "url"},
		allowed: map[string]bool{
			"author": true, "title": true, "journal": true, "year": true,
			"volume": true, "number": true, "pages": true, "doi": true, "url": true,
		},
		synonyms: map[string]string{"number": "issue"},
	},
	"book": {
		mandatory: []string{"author", "year", "title", "publisher"},
		allowed: map[string]bool{
			"author": true, "year": true, "title": true, "publisher": true,
			"address": true, "doi": true, "url": true,
		},
	},
	"incollection": {
		mandatory: []string{"author", "year", "title", "booktitle", "publisher"},
		allowed: map[string]bool{
			"author": true, "year": true, "title": true, "booktitle": true,
			"publisher": true, "address": true, "pages": true, "doi": true, "url": true,
		},
	},
	"inproceedings": {
		mandatory: []string{"author", "year", "title", "booktitle", "doi", "url"},
		allowed: map[string]bool{
			"author": true, "year": true, "title": true, "booktitle": true,
			"pages": true, "doi": true, "url": true,
		},
	},
	"phdthesis": {
		mandatory: []string{"author", "year", "title", "school", "url"},
		allowed: map[string]bool{
			"author": true, "year": true, "title": true, "school": true,
			"doi": true, "url": true,
		},
	},
	"mastersthesis": {
		mandatory: []string{"author", "year", "title", "school", "url"},
		allowed: map[string]bool{
			"author": true, "year": true, "title": true, "school": true,
			"doi": true, "url": true,
		},
	},
	"techreport": {
		mandatory: []string{"author", "year", "title", "institution", "url"},
		allowed: map[string]bool{
			"author": true, "year": true, "title": true, "institution": true,
			"doi": true, "url": true,
		},
	},
	"misc": {
		mandatory: []string{"author", "year", "title", "url"},
		allowed: map[string]bool{
			"author": true, "year": true, "title": true, "publisher": true, "doi": true, "url": true,
		},
		arxivMandatory: []string{"author", "year", "title", "eprint", "archiveprefix"},
		arxivAllowed: map[string]bool{
			"author": true, "year": true, "title": true,
			"eprint": true, "archiveprefix": true, "primaryclass": true,
		},
	},
	"unpublished": {
		mandatory: []string{"author", "title", "note"},
		allowed: map[string]bool{
			"author": true, "year": true, "title": true, "doi": true, "url": true, "note": true,
		},
	},
}

func init() {
	entrySpecs["conference"] = entrySpecs["inproceedings"]
}

// transformArxivMiscToUnpublished converts a @misc arXiv entry to @unpublished
// per IEEE style: author, year, and title are kept; eprint, archiveprefix, and
// primaryclass are dropped; a note field is added with an \href to arXiv.
func transformArxivMiscToUnpublished(e *Entry) {
	eprint := FieldValue(*e, "eprint")
	e.Type = "unpublished"
	filtered := make([]Field, 0, len(e.Fields))
	for _, f := range e.Fields {
		if f.Name == "author" || f.Name == "year" || f.Name == "title" {
			filtered = append(filtered, f)
		}
	}
	e.Fields = filtered
	note := `[arXiv preprint \href{https://arxiv.org/abs/` + eprint + `}{arXiv:` + eprint + `}]`
	SetField(e, "note", "{"+note+"}")
}

// missingMandatoryFields returns the list of mandatory field names that are
// absent (empty) on e, or nil if the entry type is unknown.
// A mandatory token of the form "a|b" is satisfied when at least one of a or b is non-empty.
func missingMandatoryFields(e Entry) []string {
	spec, ok := entrySpecs[e.Type]
	if !ok {
		return nil
	}
	mandatory := spec.mandatory
	if spec.arxivMandatory != nil && findArxivID(e) != "" {
		mandatory = spec.arxivMandatory
	}
	var missing []string
	for _, token := range mandatory {
		if a, b, ok := strings.Cut(token, "|"); ok {
			if FieldValue(e, a) == "" && FieldValue(e, b) == "" {
				missing = append(missing, token)
			}
		} else {
			if FieldValue(e, token) == "" {
				missing = append(missing, token)
			}
		}
	}
	return missing
}

// warnMissingFields returns a warning string if any mandatory fields are absent,
// or an empty string when all are present.
func warnMissingFields(e Entry) string {
	missing := missingMandatoryFields(e)
	if len(missing) == 0 {
		return ""
	}
	return "missing mandatory fields: " + strings.Join(missing, ", ")
}

// checkAPIMandatoryFields normalizes a copy of e (so url derived from doi etc.
// are reflected) and returns an error if any mandatory fields are missing.
// source is the name of the metadata provider ("Crossref" / "arXiv") used in
// the error message.
func checkAPIMandatoryFields(e Entry, source string) error {
	eCopy := e
	normalizeEntryFields(&eCopy, false)
	missing := missingMandatoryFields(eCopy)
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%s metadata missing mandatory fields for @%s: %s",
		source, eCopy.Type, strings.Join(missing, ", "))
}

// normalizeEntryFields drops non-allowed fields, resolves field synonyms,
// and derives url from doi if url is absent. Only acts on known entry types.
func normalizeEntryFields(e *Entry, urlFromDOI bool) {
	spec, ok := entrySpecs[e.Type]
	if !ok {
		return
	}

	// Select the active allowed set: arXiv override takes precedence for @misc.
	allowed := spec.allowed
	isArxiv := spec.arxivAllowed != nil && findArxivID(*e) != ""
	if isArxiv {
		allowed = spec.arxivAllowed
	}

	// Resolve synonyms before filtering so the alias is not dropped first.
	for canonical, alias := range spec.synonyms {
		if FieldValue(*e, canonical) == "" {
			if val := FieldValue(*e, alias); val != "" {
				SetField(e, canonical, "{"+val+"}")
			}
		}
	}
	// Drop fields not in the allowed set (including consumed alias fields),
	// and placeholder DOIs (10.5555/…) together with their doi.org url.
	filtered := make([]Field, 0, len(e.Fields))
	for _, f := range e.Fields {
		if f.Name == "doi" && isPlaceholderDOI(FieldValue(*e, "doi")) {
			continue
		}
		if f.Name == "url" && isPlaceholderDOIURL(FieldValue(*e, "url")) {
			continue
		}
		if allowed[f.Name] {
			filtered = append(filtered, f)
		}
	}
	e.Fields = filtered
	// arXiv @misc entries always carry archiveprefix = {arXiv}.
	if isArxiv {
		SetField(e, "archiveprefix", "{arXiv}")
	}
	// Derive url from doi when the type supports both.
	// Not applied to arXiv @misc entries, which are identified by eprint.
	// When urlFromDOI is true, replace any existing url; otherwise only set when url is absent.
	if !isArxiv && allowed["url"] {
		if doi := FieldValue(*e, "doi"); doi != "" {
			if urlFromDOI || FieldValue(*e, "url") == "" {
				SetField(e, "url", "{https://doi.org/"+doi+"}")
			}
		}
	}
	// Upgrade http:// to https:// in the url field.
	if u := FieldValue(*e, "url"); strings.HasPrefix(u, "http://") {
		SetField(e, "url", "{https://"+u[len("http://"):]+"}")
	}
}

// ensureArticleOptionalFields adds blank placeholders for volume, number, and
// pages in @article entries if those fields are not already present.
func ensureArticleOptionalFields(e *Entry) {
	if e.Type != "article" {
		return
	}
	for _, name := range []string{"volume", "number", "pages"} {
		if FieldValue(*e, name) == "" {
			SetField(e, name, "{}")
		}
	}
}

// findDOI returns the entry's DOI from the doi field or a doi.org url.
// Placeholder DOIs (see isPlaceholderDOI) and arXiv DOIs (handled by
// findArxivID) are ignored.
func findDOI(e Entry) string {
	if doi := FieldValue(e, "doi"); doi != "" && !isPlaceholderDOI(doi) && arxivIDFromDOI(doi) == "" {
		return doi
	}
	u := FieldValue(e, "url")
	if _, after, ok := strings.Cut(u, "doi.org/"); ok && !isPlaceholderDOI(after) && arxivIDFromDOI(after) == "" {
		return after
	}
	return ""
}

// entryRawDOI returns the doi field, else the DOI of a doi.org url, without
// any filtering.
func entryRawDOI(e Entry) string {
	if doi := FieldValue(e, "doi"); doi != "" {
		return doi
	}
	if _, after, ok := strings.Cut(FieldValue(e, "url"), "doi.org/"); ok {
		return after
	}
	return ""
}

// isPlaceholderDOI reports whether doi uses Crossref's test prefix 10.5555.
// No real publication has such a DOI; the ACM Digital Library uses it for
// pseudo-DOIs on items without a registered DOI (e.g. 10.5555/1671238).
func isPlaceholderDOI(doi string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(doi)), "10.5555/")
}

// isPlaceholderDOIURL reports whether u is a doi.org URL of a placeholder DOI.
func isPlaceholderDOIURL(u string) bool {
	_, after, ok := strings.Cut(u, "doi.org/")
	return ok && isPlaceholderDOI(after)
}

var reArxivURL = regexp.MustCompile(`arxiv\.org/abs/([0-9]{4}\.[0-9]{4,5}(?:v\d+)?|[a-z-]+/[0-9]{7})`)
var reArxivBare = regexp.MustCompile(`^([0-9]{4}\.[0-9]{4,5}(?:v\d+)?|[a-z-]+/[0-9]{7})$`)

func findArxivID(e Entry) string {
	eprint := FieldValue(e, "eprint")
	if eprint != "" {
		ap := strings.ToLower(FieldValue(e, "archiveprefix"))
		et := strings.ToLower(FieldValue(e, "eprinttype"))
		if ap == "arxiv" || et == "arxiv" {
			return eprint
		}
	}
	if m := reArxivURL.FindStringSubmatch(strings.ToLower(FieldValue(e, "url"))); m != nil {
		return m[1]
	}
	if id := arxivIDFromDOI(FieldValue(e, "doi")); id != "" {
		return id
	}
	if _, after, ok := strings.Cut(FieldValue(e, "url"), "doi.org/"); ok {
		return arxivIDFromDOI(after)
	}
	return ""
}

// ── helpers ───────────────────────────────────────────────────────────────────

// applyField sets a field to value if it differs from the current value.
// Returns true if the entry was modified.
func applyField(e *Entry, name, value string) bool {
	if normalizeFieldValue(FieldValue(*e, name)) == normalizeFieldValue(value) {
		return false
	}
	SetField(e, name, "{"+value+"}")
	return true
}

func normalizeFieldValue(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// ErrUnrecognizedID is returned by AddEntryFromID when the given string is
// neither a DOI nor an arXiv identifier.
var ErrUnrecognizedID = fmt.Errorf("not a valid DOI or arXiv identifier")

// normalizeDOI strips common URL prefixes and returns the bare DOI (starting
// with "10.") if s is a valid DOI, otherwise empty string.
func normalizeDOI(s string) string {
	low := strings.ToLower(s)
	for _, prefix := range []string{"https://doi.org/", "http://doi.org/", "doi.org/"} {
		if strings.HasPrefix(low, prefix) {
			s = s[len(prefix):]
			break
		}
	}
	if strings.HasPrefix(s, "10.") && strings.ContainsRune(s, '/') {
		return s
	}
	return ""
}

// NormalizeArxivID returns the bare arXiv identifier from s (URL or bare form),
// or empty string if s is not an arXiv identifier.
func NormalizeArxivID(s string) string { return normalizeArxivID(s) }

// normalizeArxivID returns the bare arXiv identifier from s (URL or bare form),
// or empty string if s is not an arXiv identifier.
func normalizeArxivID(s string) string {
	if m := reArxivURL.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	if m := reArxivBare.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// AddEntryFromID fetches metadata for a DOI or arXiv ID and inserts the entry
// into the global bib cache. Returns the canonical cite key on success.
// Returns ErrUnrecognizedID if s is neither a DOI nor an arXiv identifier.
// If the entry is already cached, its existing key is returned with isNew=false.
func AddEntryFromID(id string, log Logger) (key string, isNew bool, err error) {
	log = logOrNop(log)
	err = withGlobalLock(func() error {
		key, isNew, err = addEntryFromIDLocked(id, log)
		return err
	})
	return key, isNew, err
}

func addEntryFromIDLocked(id string, log Logger) (key string, isNew bool, err error) {
	c, err := loadCacheStrict()
	if err != nil {
		return "", false, err
	}
	e, err := entryFromID(id)
	if err != nil {
		return "", false, err
	}
	idx := newIDIndex(c, false)
	inputID := entryID(e)
	if key, ok := idx[inputID]; ok {
		return key, false, nil
	}
	res, err := resolveEntry(e, func(id string) bool { _, ok := idx[id]; return ok }, log)
	if err != nil {
		return "", false, err
	}
	for _, id := range res.ids() {
		if key, ok := idx[id]; ok {
			if entry := c[key]; entry.addAliases(inputID) {
				c[key] = entry
				saveCache(c, log)
			}
			return key, false, nil
		}
	}
	if err := checkAPIMandatoryFields(res.entry, sourceLabel[res.source]); err != nil {
		return "", false, err
	}
	key = disambiguateKey(GenerateKey(res.entry), c)
	entry := buildCacheEntry(res.entry, res.raw, res.source, "")
	entry.addAliases(append(res.ids(), inputID)...)
	c[key] = entry
	saveCache(c, log)
	return key, true, nil
}

// disambiguateKey returns key if it is not already in c, otherwise appends a
// lowercase letter suffix (a, b, c, …) until a free slot is found.
func disambiguateKey(key string, c cache) string {
	if key == "" || key == "tmp" {
		key = "entry"
	}
	if _, exists := c[key]; !exists {
		return key
	}
	for i := 'a'; i <= 'z'; i++ {
		candidate := key + string(i)
		if _, exists := c[candidate]; !exists {
			return candidate
		}
	}
	return key
}
