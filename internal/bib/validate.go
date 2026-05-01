package bib

import (
	"errors"
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
func AllocateCacheEntries(bibFiles []string, auxDir string, retryTimeout bool, log Logger) (int, map[string]string, error) {
	log = logOrNop(log)
	if len(bibFiles) == 0 {
		return 0, map[string]string{}, nil
	}
	c, err := loadCacheStrict(auxDir)
	if err != nil {
		return 0, nil, err
	}

	// Build reverse-index of IDs already in cache for fast dedup.
	// When retryTimeout is true, exclude "timeout" entries so they get re-validated.
	cachedDOIs := make(map[string]bool)
	cachedArxivIDs := make(map[string]bool)
	for _, entry := range c {
		if retryTimeout && entry.Source == "timeout" {
			continue
		}
		if doi := entry.Fields["doi"]; doi != "" {
			cachedDOIs[strings.ToLower(doi)] = true
		}
		if id := entry.Fields["eprint"]; id != "" {
			cachedArxivIDs[strings.ToLower(id)] = true
		}
	}

	added := 0
	allRenames := map[string]string{}
	for _, path := range bibFiles {
		n, renames, err := allocateBibFile(path, c, cachedDOIs, cachedArxivIDs, log)
		if err != nil {
			return added, allRenames, err
		}
		added += n
		maps.Copy(allRenames, renames)
	}
	if added > 0 {
		saveCache(auxDir, c, log)
	}
	return added, allRenames, nil
}

func allocateBibFile(path string, c cache, cachedDOIs, cachedArxivIDs map[string]bool, log Logger) (int, map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, fmt.Errorf("cannot read %s: %w", path, err)
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

	for i, item := range items {
		if !item.IsEntry {
			continue
		}
		e := item.Entry
		for j, f := range e.Fields {
			e.Fields[j].Value = unescapeFieldValue(f.Value)
		}
		rawURL := FieldValue(e, "url")
		doi := findDOI(e)
		arxivID := findArxivID(e)

		switch {
		case doi != "":
			if cachedDOIs[strings.ToLower(doi)] {
				continue
			}
			corrected, raw, source, warn := validateEntry(e, false, log)
			if warn != "" {
				log.Warn(e.Key, warn)
			}
			if corrected != nil {
				items[i].Entry = *corrected
			}
			pending = append(pending, pendingEntry{i, buildCacheEntry(items[i].Entry, raw, source, rawURL)})
			cachedDOIs[strings.ToLower(doi)] = true
			added++
		case arxivID != "":
			if cachedArxivIDs[strings.ToLower(arxivID)] {
				continue
			}
			corrected, raw, source, warn := validateEntry(e, false, log)
			if warn != "" {
				log.Warn(e.Key, warn)
			}
			if corrected != nil {
				items[i].Entry = *corrected
			}
			pending = append(pending, pendingEntry{i, buildCacheEntry(items[i].Entry, raw, source, rawURL)})
			cachedArxivIDs[strings.ToLower(arxivID)] = true
			added++
		default:
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
			fields := make(map[string]string, len(e.Fields))
			for _, f := range e.Fields {
				if v := FieldValue(e, f.Name); v != "" {
					fields[f.Name] = v
				}
			}
			delete(fields, "url")
			if rawURL != "" {
				fields["url"] = rawURL
			}
			items[i].Entry = e
			pending = append(pending, pendingEntry{i, cacheEntry{Source: "no-id", Type: e.Type, Fields: fields}})
			added++
		}
	}

	// Re-assign canonical keys: Crossref/arXiv may have corrected author/title/year.
	assignCanonicalKeys(items)
	for _, p := range pending {
		c[items[p.itemIdx].Entry.Key] = p.entry
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
	return added, renames, nil
}

// buildCacheEntry constructs a cacheEntry from a validated entry and its raw API
// response, mirroring the snapshot logic in processBibFile.
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
	if source == "crossref" || source == "arxiv" {
		for k, v := range raw.Fields {
			if v != "" {
				fields[k] = v
			}
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
func WriteBibFromCache(path string, citeKeys []string, auxDir string, opts WriteOptions) error {
	if len(citeKeys) == 0 {
		return nil
	}
	c := loadCache(auxDir)

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
			"author": true, "year": true, "title": true, "doi": true, "url": true,
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

// warnMissingFields returns a warning string if any mandatory fields are absent,
// or an empty string when all are present.
// A mandatory token of the form "a|b" is satisfied when at least one of a or b is non-empty.
func warnMissingFields(e Entry) string {
	spec, ok := entrySpecs[e.Type]
	if !ok {
		return ""
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
	if len(missing) > 0 {
		return "missing mandatory fields: " + strings.Join(missing, ", ")
	}
	return ""
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
	// Drop fields not in the allowed set (including consumed alias fields).
	filtered := make([]Field, 0, len(e.Fields))
	for _, f := range e.Fields {
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

// validateEntry looks up the entry via Crossref or arXiv and returns a
// corrected entry (nil if nothing changed), the raw fields for caching, the
// source used, and an optional warning.
func validateEntry(e Entry, abbreviateJournals bool, log Logger) (corrected *Entry, raw cacheEntry, source, warning string) {
	log = logOrNop(log)
	if doi := findDOI(e); doi != "" {
		result, raw, crType, err := queryCrossref(e, doi, log)
		if err != nil {
			source := "timeout"
			if errors.Is(err, errNotFound) {
				source = "invalid-id"
			}
			return nil, cacheEntry{}, source, fmt.Sprintf("Crossref query failed: %v", err)
		}
		if crType != "" {
			if result != nil {
				result.Type = crType
			} else {
				cp := e
				result = &cp
				result.Type = crType
			}
		}
		// Apply journal abbreviation to the corrected entry (raw.Fields["journal"] is always full).
		if result != nil && raw.Fields["journal"] != "" && abbreviateJournals {
			SetField(result, "journal", "{"+AbbreviateISO4(raw.Fields["journal"])+"}")
		}
		return result, raw, "crossref", ""
	}

	if id := findArxivID(e); id != "" {
		result, raw, doi, err := queryArxiv(e, id, log)
		if err != nil {
			source := "timeout"
			if errors.Is(err, errNotFound) {
				source = "invalid-id"
			}
			return nil, cacheEntry{}, source, fmt.Sprintf("arXiv query failed: %v", err)
		}
		if doi != "" {
			// arXiv entry has a DOI — use Crossref validation instead.
			log.Info(e.Key, fmt.Sprintf("arXiv entry has DOI %s, using Crossref", doi))
			crResult, crRaw, crType, crErr := queryCrossref(e, doi, log)
			if crErr != nil {
				// Crossref failed — fall back to arXiv result.
				log.Warn(e.Key, fmt.Sprintf("Crossref query failed (%v), falling back to arXiv", crErr))
				return result, raw, "arxiv", ""
			}
			entryType := crType
			if entryType == "" {
				entryType = "article"
			}
			if crResult != nil {
				crResult.Type = entryType
			} else {
				cp := e
				crResult = &cp
				crResult.Type = entryType
			}
			if crRaw.Fields["journal"] != "" && abbreviateJournals {
				SetField(crResult, "journal", "{"+AbbreviateISO4(crRaw.Fields["journal"])+"}")
			}
			return crResult, crRaw, "crossref", ""
		}
		return result, raw, "arxiv", ""
	}

	if spec, ok := entrySpecs[e.Type]; ok && slices.Contains(spec.mandatory, "doi") {
		return nil, cacheEntry{}, "no-id", "no DOI or arXiv ID — skipping validation"
	}
	return nil, cacheEntry{}, "no-id", ""
}

func findDOI(e Entry) string {
	if doi := FieldValue(e, "doi"); doi != "" {
		return doi
	}
	u := FieldValue(e, "url")
	if _, after, ok := strings.Cut(u, "doi.org/"); ok {
		return after
	}
	return ""
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
// into the bib cache at auxDir. Returns the canonical cite key on success.
// Returns ErrUnrecognizedID if s is neither a DOI nor an arXiv identifier.
// If the entry is already cached, its existing key is returned with isNew=false.
func AddEntryFromID(id, auxDir string, log Logger) (key string, isNew bool, err error) {
	log = logOrNop(log)
	c, err := loadCacheStrict(auxDir)
	if err != nil {
		return "", false, err
	}

	if doi := normalizeDOI(id); doi != "" {
		for key, entry := range c {
			if strings.EqualFold(entry.Fields["doi"], doi) {
				return key, false, nil
			}
		}
		base := Entry{Type: "article", Key: "tmp", Fields: []Field{{Name: "doi", Value: doi}}}
		corrected, raw, crType, err := queryCrossref(base, doi, log)
		if err != nil {
			return "", false, err
		}
		e := base
		if corrected != nil {
			e = *corrected
		}
		if crType != "" {
			e.Type = crType
		}
		raw.Fields["doi"] = strings.ToLower(doi)
		cEntry := buildCacheEntry(e, raw, "crossref", "")
		key := disambiguateKey(GenerateKey(e), c)
		c[key] = cEntry
		saveCache(auxDir, c, log)
		return key, true, nil
	}

	if arxivID := normalizeArxivID(id); arxivID != "" {
		for key, entry := range c {
			if strings.EqualFold(entry.Fields["eprint"], arxivID) {
				return key, false, nil
			}
		}
		base := Entry{
			Type: "misc",
			Key:  "tmp",
			Fields: []Field{
				{Name: "eprint", Value: arxivID},
				{Name: "archiveprefix", Value: "{arXiv}"},
			},
		}
		corrected, raw, doi, err := queryArxiv(base, arxivID, log)
		if err != nil {
			return "", false, err
		}

		if doi != "" {
			// arXiv entry has a DOI — use Crossref instead.
			for key, entry := range c {
				if strings.EqualFold(entry.Fields["doi"], doi) {
					return key, false, nil
				}
			}
			crBase := Entry{Type: "article", Key: "tmp", Fields: []Field{{Name: "doi", Value: doi}}}
			crCorrected, crRaw, crType, crErr := queryCrossref(crBase, doi, log)
			if crErr == nil {
				e := crBase
				if crCorrected != nil {
					e = *crCorrected
				}
				if crType != "" {
					e.Type = crType
				}
				crRaw.Fields["doi"] = strings.ToLower(doi)
				cEntry := buildCacheEntry(e, crRaw, "crossref", "")
				key := disambiguateKey(GenerateKey(e), c)
				c[key] = cEntry
				saveCache(auxDir, c, log)
				return key, true, nil
			}
			// Crossref failed — fall back to arXiv result.
			log.Warn("", fmt.Sprintf("Crossref failed for DOI %s (%v), falling back to arXiv", doi, crErr))
		}

		e := base
		if corrected != nil {
			e = *corrected
		}
		raw.Fields["eprint"] = arxivID
		cEntry := buildCacheEntry(e, raw, "arxiv", "")
		key := disambiguateKey(GenerateKey(e), c)
		c[key] = cEntry
		saveCache(auxDir, c, log)
		return key, true, nil
	}

	return "", false, ErrUnrecognizedID
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
