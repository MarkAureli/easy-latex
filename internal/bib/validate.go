package bib

import (
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// cacheEntry stores the validation source and a snapshot of all allowed field
// values for an entry. Fields holds pre-config-transform values so that changes
// to abbreviateJournals, maxAuthors, abbreviateFirstName, braceTitles, or
// urlFromDOI take effect on the next compile without re-fetching from the API.
//
// API-sourced fields stored raw:
//   - title: after stripNonEscapedBraces (non-configurable), before brace-wrapping
//   - author: full "Last, First and …" string, before maxAuthors/abbreviation
//   - journal: full unabbreviated name (Crossref only)
//   - url: as present in the bib file before urlFromDOI; absent when not in bib
//
// All other allowed fields (year, volume, number, pages, doi, booktitle, …) are
// stored as returned by the API or as present in the bib file.
type cacheEntry struct {
	Source string            `json:"source"`
	Type   string            `json:"type"`
	Fields map[string]string `json:"fields,omitempty"`
}

// cache maps canonical citation keys to their cacheEntry.
type cache map[string]cacheEntry

func loadCache(auxDir string) cache {
	data, err := os.ReadFile(filepath.Join(auxDir, "bib.json"))
	if err != nil {
		return make(cache)
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil {
		// Unreadable or old-format cache: start fresh; entries will be re-validated.
		return make(cache)
	}
	// Remove entries that lack a Type: these are old-format entries that predate
	// the Type field. They will be re-allocated on the next parsebib run.
	for k, v := range c {
		if v.Type == "" {
			delete(c, k)
		}
	}
	return c
}

func saveCache(auxDir string, c cache) {
	data, _ := json.MarshalIndent(c, "", "  ")
	_ = os.WriteFile(filepath.Join(auxDir, "bib.json"), data, 0644)
}

// LoadRenames reads .el/renames.json and returns the old→new key map.
// Returns an empty map if the file is absent or unreadable.
func LoadRenames(auxDir string) map[string]string {
	data, err := os.ReadFile(filepath.Join(auxDir, "renames.json"))
	if err != nil {
		return map[string]string{}
	}
	var r map[string]string
	if err := json.Unmarshal(data, &r); err != nil {
		return map[string]string{}
	}
	return r
}

// SaveRenames merges renames into .el/renames.json (creates if absent).
func SaveRenames(auxDir string, renames map[string]string) {
	if len(renames) == 0 {
		return
	}
	existing := LoadRenames(auxDir)
	for k, v := range renames {
		existing[k] = v
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	_ = os.WriteFile(filepath.Join(auxDir, "renames.json"), data, 0644)
}

// ClearRenames removes .el/renames.json.
func ClearRenames(auxDir string) {
	_ = os.Remove(filepath.Join(auxDir, "renames.json"))
}

// BibFileChanged reports whether bibPath has changed since the last
// UpdateBibHash call. Returns false if the file cannot be read.
func BibFileChanged(bibPath, auxDir string) bool {
	data, err := os.ReadFile(bibPath)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum) != loadBibHash(auxDir)
}

// UpdateBibHash computes the SHA-256 hash of bibPath and saves it to
// auxDir/bib_hash.
func UpdateBibHash(bibPath, auxDir string) {
	data, err := os.ReadFile(bibPath)
	if err != nil {
		return
	}
	sum := sha256.Sum256(data)
	_ = os.WriteFile(filepath.Join(auxDir, "bib_hash"), []byte(fmt.Sprintf("%x", sum)), 0644)
}

func loadBibHash(auxDir string) string {
	data, err := os.ReadFile(filepath.Join(auxDir, "bib_hash"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

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
func AllocateCacheEntries(bibFiles []string, auxDir string) (int, map[string]string, error) {
	if len(bibFiles) == 0 {
		return 0, map[string]string{}, nil
	}
	c := loadCache(auxDir)

	// Build reverse-index of IDs already in cache for fast dedup.
	cachedDOIs := make(map[string]bool)
	cachedArxivIDs := make(map[string]bool)
	for _, entry := range c {
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
		n, renames, err := allocateBibFile(path, c, cachedDOIs, cachedArxivIDs)
		if err != nil {
			return added, allRenames, err
		}
		added += n
		for k, v := range renames {
			allRenames[k] = v
		}
	}
	if added > 0 {
		saveCache(auxDir, c)
	}
	return added, allRenames, nil
}

func allocateBibFile(path string, c cache, cachedDOIs, cachedArxivIDs map[string]bool) (int, map[string]string, error) {
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
		rawURL := FieldValue(e, "url")
		doi := findDOI(e)
		arxivID := findArxivID(e)

		switch {
		case doi != "":
			if cachedDOIs[strings.ToLower(doi)] {
				continue
			}
			corrected, raw, source, warn := validateEntry(e, false)
			if warn != "" {
				fmt.Printf("[bib] %s: %s\n", e.Key, warn)
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
			corrected, raw, source, warn := validateEntry(e, false)
			if warn != "" {
				fmt.Printf("[bib] %s: %s\n", e.Key, warn)
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
				fmt.Printf("[bib] %s: %s\n", e.Key, warn)
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
	if source == "crossref" || source == "arxiv" {
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
	}
	return entry
}

// LoadCacheKeys returns all canonical citation keys stored in the bib cache.
func LoadCacheKeys(auxDir string) []string {
	c := loadCache(auxDir)
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// WriteBibFromCache generates path from the bib cache for the given cited keys,
// applying all config transforms. Returns an error if any key is absent from
// the cache (caller should run 'el parsebib' first).
func WriteBibFromCache(path string, citeKeys []string, auxDir string, abbreviateJournals, braceTitles, ieeeFormat bool, maxAuthors int, abbreviateFirstName, urlFromDOI bool) error {
	if len(citeKeys) == 0 {
		return nil
	}
	c := loadCache(auxDir)

	items := make([]Item, 0, len(citeKeys))
	for _, key := range citeKeys {
		cached, ok := c[key]
		if !ok {
			return fmt.Errorf("cite key %q not found in bib cache; run 'el parsebib'", key)
		}

		e := Entry{Key: key, Type: cached.Type}
		for name, val := range cached.Fields {
			if name == "journal" && abbreviateJournals && cached.Source == "crossref" {
				SetField(&e, name, "{"+AbbreviateISO4(val)+"}")
			} else {
				SetField(&e, name, "{"+val+"}")
			}
		}

		normalizeEntryFields(&e, urlFromDOI)

		if ieeeFormat && e.Type == "misc" && findArxivID(e) != "" {
			transformArxivMiscToUnpublished(&e)
		}

		if author := FieldValue(e, "author"); author != "" {
			SetField(&e, "author", "{"+formatAuthorField(author, maxAuthors, abbreviateFirstName)+"}")
		}

		if title := FieldValue(e, "title"); title != "" {
			if normalized := stripNonEscapedBraces(title); normalized != title {
				SetField(&e, "title", "{"+normalized+"}")
			}
		}

		if braceTitles || ieeeFormat {
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

// ProcessBibFiles formats and validates every registered .bib file.
// It returns a map of renamed citation keys (oldKey → newKey) across all files,
// which the caller can use to update \cite{} references in .tex sources.
func ProcessBibFiles(bibFiles []string, auxDir string, abbreviateJournals, braceTitles, ieeeFormat bool, maxAuthors int, abbreviateFirstName, urlFromDOI bool) (map[string]string, error) {
	if len(bibFiles) == 0 {
		return nil, nil
	}
	c := loadCache(auxDir)
	cacheChanged := false
	allRenames := make(map[string]string)

	for _, path := range bibFiles {
		renames, changed, err := processBibFile(path, auxDir, c, abbreviateJournals, braceTitles, ieeeFormat, maxAuthors, abbreviateFirstName, urlFromDOI)
		if err != nil {
			return nil, err
		}
		if changed {
			cacheChanged = true
		}
		for old, new := range renames {
			allRenames[old] = new
		}
	}

	if cacheChanged {
		saveCache(auxDir, c)
	}
	return allRenames, nil
}

func processBibFile(path, auxDir string, c cache, abbreviateJournals, braceTitles, ieeeFormat bool, maxAuthors int, abbreviateFirstName, urlFromDOI bool) (renames map[string]string, cacheChanged bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("cannot read %s: %w", path, err)
	}

	items := ParseFile(string(data))

	// Capture original keys before any renaming so we can build the rename map.
	origKeys := make(map[int]string)
	for i, item := range items {
		if item.IsEntry {
			origKeys[i] = item.Entry.Key
		}
	}

	assignCanonicalKeys(items)

	// pendingCache collects entries validated this run.
	// Written to the cache after the second assignCanonicalKeys so the cache
	// key matches the final canonical key.
	type pendingEntry struct {
		itemIdx int
		entry   cacheEntry
	}
	var pending []pendingEntry

	for i, item := range items {
		if !item.IsEntry {
			continue
		}
		e := item.Entry

		// Capture raw url before any modification; stored as-is in the cache
		// so that urlFromDOI is always re-applied from current config.
		rawURL := FieldValue(e, "url")

		// Capture original field values to compare after post-processing.
		origAuthor := FieldValue(e, "author")
		origTitle := FieldValue(e, "title")
		origJournal := FieldValue(e, "journal")
		origYear := FieldValue(e, "year")
		origVolume := FieldValue(e, "volume")
		origNumber := FieldValue(e, "number")
		origPages := FieldValue(e, "pages")
		origDOI := FieldValue(e, "doi")

		var validationSource string
		cached, seen := c[e.Key]
		if !seen {
			corrected, raw, source, warn := validateEntry(e, abbreviateJournals)
			if warn != "" {
				fmt.Printf("[bib] %s: %s\n", e.Key, warn)
			}
			if corrected != nil {
				e = *corrected
			}
			if source != "" {
				entry := cacheEntry{Source: source}
				if source == "crossref" || source == "arxiv" {
					// Build a full snapshot of all allowed fields for this entry.
					// Normalize a copy to get canonical field names and drop unknown fields,
					// but skip urlFromDOI so we preserve the raw url from the bib file.
					eCopy := e
					normalizeEntryFields(&eCopy, false)
					fields := make(map[string]string, len(eCopy.Fields))
					for _, f := range eCopy.Fields {
						if v := FieldValue(eCopy, f.Name); v != "" {
							fields[f.Name] = v
						}
					}
					// Overlay raw API values (pre-config-transform: full journal name,
					// unformatted authors, pre-brace title, pre-abbreviation year/vol/etc).
					for k, v := range raw.Fields {
						if v != "" {
							fields[k] = v
						}
					}
					// Restore raw url: store what was in the bib file, not any
					// doi-derived value. urlFromDOI is re-applied each compile.
					delete(fields, "url")
					if rawURL != "" {
						fields["url"] = rawURL
					}
					entry.Fields = fields
				}
				pending = append(pending, pendingEntry{i, entry})
				cacheChanged = true
			}
			if source == "crossref" || source == "arxiv" {
				validationSource = source
			}
		} else if cached.Source == "crossref" || cached.Source == "arxiv" {
			// Re-apply all cached fields so that changes to abbreviateJournals,
			// maxAuthors, abbreviateFirstName, braceTitles, or urlFromDOI take
			// effect on the next compile without re-fetching from the API.
			// journal is re-abbreviated here; all other config transforms run
			// later in the pipeline (normalizeEntryFields, author formatting, etc).
			for name, val := range cached.Fields {
				if name == "journal" && abbreviateJournals {
					SetField(&e, name, "{"+AbbreviateISO4(val)+"}")
				} else {
					SetField(&e, name, "{"+val+"}")
				}
			}
		}

		normalizeEntryFields(&e, urlFromDOI)

		if ieeeFormat && e.Type == "misc" && findArxivID(e) != "" {
			transformArxivMiscToUnpublished(&e)
		}

		if author := FieldValue(e, "author"); author != "" {
			SetField(&e, "author", "{"+formatAuthorField(author, maxAuthors, abbreviateFirstName)+"}")
		}

		if title := FieldValue(e, "title"); title != "" {
			if normalized := stripNonEscapedBraces(title); normalized != title {
				SetField(&e, "title", "{"+normalized+"}")
			}
		}

		if braceTitles || ieeeFormat {
			if title := FieldValue(e, "title"); title != "" {
				SetField(&e, "title", "{{"+title+"}}")
			}
		}

		if warn := warnMissingFields(e); warn != "" {
			fmt.Printf("[bib] %s: %s\n", e.Key, warn)
		}

		ensureArticleOptionalFields(&e)

		// Sort fields after validation so any newly added fields are ordered too.
		e.Fields = sortedFields(e.Type, e.Fields)

		// Log only fields that actually changed in the output after all post-processing.
		if validationSource != "" {
			type fieldCheck struct {
				name string
				orig string
			}
			checks := []fieldCheck{
				{"author", origAuthor},
				{"title", origTitle},
				{"journal", origJournal},
				{"year", origYear},
				{"volume", origVolume},
				{"number", origNumber},
				{"pages", origPages},
				{"doi", origDOI},
			}
			var corrections []string
			for _, fc := range checks {
				if normalizeFieldValue(FieldValue(e, fc.name)) != normalizeFieldValue(fc.orig) {
					corrections = append(corrections, fc.name)
				}
			}
			if len(corrections) > 0 {
				fmt.Printf("[bib] %s: reformatted %s\n", e.Key, strings.Join(corrections, ", "))
			}
		}

		items[i].Entry = e
	}

	// Re-assign keys: Crossref/arXiv may have populated author, year, or title
	// for entries that previously fell back to their original key.
	assignCanonicalKeys(items)

	// Flush pending cache entries under the final canonical keys.
	for _, p := range pending {
		c[items[p.itemIdx].Entry.Key] = p.entry
	}

	// Build rename map: entries whose key changed from the original .bib key.
	renames = make(map[string]string)
	for i, item := range items {
		if !item.IsEntry {
			continue
		}
		if orig := origKeys[i]; orig != item.Entry.Key {
			renames[orig] = item.Entry.Key
		}
	}

	formatted := renderItems(items)
	if formatted != string(data) {
		if err := os.WriteFile(path, []byte(formatted), 0644); err != nil {
			return renames, cacheChanged, fmt.Errorf("cannot write %s: %w", path, err)
		}
	}
	return renames, cacheChanged, nil
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
	"conference": {
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
func validateEntry(e Entry, abbreviateJournals bool) (corrected *Entry, raw cacheEntry, source, warning string) {
	if doi := findDOI(e); doi != "" {
		result, raw, err := queryCrossref(e, doi)
		if err != nil {
			return nil, cacheEntry{}, "", fmt.Sprintf("Crossref query failed: %v", err)
		}
		// Apply journal abbreviation to the corrected entry (raw.Fields["journal"] is always full).
		if result != nil && raw.Fields["journal"] != "" && abbreviateJournals {
			SetField(result, "journal", "{"+AbbreviateISO4(raw.Fields["journal"])+"}")
		}
		return result, raw, "crossref", ""
	}

	if id := findArxivID(e); id != "" {
		result, raw, err := queryArxiv(e, id)
		if err != nil {
			return nil, cacheEntry{}, "", fmt.Sprintf("arXiv query failed: %v", err)
		}
		return result, raw, "arxiv", ""
	}

	if doiIsMandatory(e.Type) {
		return nil, cacheEntry{}, "no-id", "no DOI or arXiv ID — skipping validation"
	}
	return nil, cacheEntry{}, "no-id", ""
}

// doiIsMandatory reports whether "doi" is listed as a mandatory field for the
// given entry type.
func doiIsMandatory(entryType string) bool {
	spec, ok := entrySpecs[entryType]
	if !ok {
		return false
	}
	for _, m := range spec.mandatory {
		if m == "doi" {
			return true
		}
	}
	return false
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

// ── Crossref ──────────────────────────────────────────────────────────────────

type crossrefAuthor struct {
	Family string `json:"family"`
	Given  string `json:"given"`
}

type crossrefResponse struct {
	Status  string `json:"status"`
	Message struct {
		Title          []string         `json:"title"`
		Author         []crossrefAuthor `json:"author"`
		ContainerTitle []string         `json:"container-title"`
		Published      struct {
			DateParts [][]int `json:"date-parts"`
		} `json:"published"`
		Volume string `json:"volume"`
		Issue  string `json:"issue"`
		Page   string `json:"page"`
		DOI    string `json:"DOI"`
	} `json:"message"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

func queryCrossref(e Entry, doi string) (*Entry, cacheEntry, error) {
	req, err := http.NewRequest("GET", "https://api.crossref.org/works/"+url.PathEscape(doi), nil)
	if err != nil {
		return nil, cacheEntry{}, err
	}
	req.Header.Set("User-Agent", "easy-latex/0.1 (https://github.com/MarkAureli/easy-latex)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, cacheEntry{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, cacheEntry{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, cacheEntry{}, err
	}

	var cr crossrefResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, cacheEntry{}, err
	}
	if cr.Status != "ok" {
		return nil, cacheEntry{}, fmt.Errorf("status: %s", cr.Status)
	}

	m := cr.Message
	updated := e
	raw := cacheEntry{Fields: make(map[string]string)}
	var corrections []string

	if len(m.Title) > 0 {
		// Strip non-escaped braces here (non-configurable preprocessing) so the
		// cached title is already in its final pre-transformation state.
		raw.Fields["title"] = stripNonEscapedBraces(m.Title[0])
		if applyField(&updated, "title", raw.Fields["title"]) {
			corrections = append(corrections, "title")
		}
	}
	if len(m.Author) > 0 {
		raw.Fields["author"] = formatCrossrefAuthors(m.Author)
		if applyField(&updated, "author", raw.Fields["author"]) {
			corrections = append(corrections, "author")
		}
	}
	if len(m.ContainerTitle) > 0 {
		// Store the full journal name; abbreviation is applied by the caller.
		raw.Fields["journal"] = m.ContainerTitle[0]
		if applyField(&updated, "journal", raw.Fields["journal"]) {
			corrections = append(corrections, "journal")
		}
	}
	if len(m.Published.DateParts) > 0 && len(m.Published.DateParts[0]) > 0 {
		raw.Fields["year"] = fmt.Sprintf("%d", m.Published.DateParts[0][0])
		if applyField(&updated, "year", raw.Fields["year"]) {
			corrections = append(corrections, "year")
		}
	}
	if m.Volume != "" {
		raw.Fields["volume"] = m.Volume
		if applyField(&updated, "volume", m.Volume) {
			corrections = append(corrections, "volume")
		}
	}
	if m.Issue != "" {
		raw.Fields["number"] = m.Issue
		if applyField(&updated, "number", m.Issue) {
			corrections = append(corrections, "number")
		}
	}
	if m.Page != "" {
		raw.Fields["pages"] = m.Page
		if applyField(&updated, "pages", m.Page) {
			corrections = append(corrections, "pages")
		}
	}
	if m.DOI != "" {
		raw.Fields["doi"] = strings.ToLower(m.DOI)
		if applyField(&updated, "doi", raw.Fields["doi"]) {
			corrections = append(corrections, "doi")
		}
	}

	if len(corrections) > 0 {
		return &updated, raw, nil
	}
	return nil, raw, nil
}

func formatCrossrefAuthors(authors []crossrefAuthor) string {
	parts := make([]string, 0, len(authors))
	for _, a := range authors {
		switch {
		case a.Family != "" && a.Given != "":
			parts = append(parts, a.Family+", "+a.Given)
		case a.Family != "":
			parts = append(parts, a.Family)
		default:
			parts = append(parts, a.Given)
		}
	}
	return strings.Join(parts, " and ")
}

// ── arXiv ─────────────────────────────────────────────────────────────────────

type arxivFeed struct {
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	Title           string        `xml:"title"`
	Authors         []arxivAuthor `xml:"author"`
	Published       string        `xml:"published"`
	PrimaryCategory struct {
		Term string `xml:"term,attr"`
	} `xml:"http://arxiv.org/schemas/atom primary_category"`
}

type arxivAuthor struct {
	Name string `xml:"name"`
}

func queryArxiv(e Entry, id string) (*Entry, cacheEntry, error) {
	resp, err := httpClient.Get("https://export.arxiv.org/api/query?id_list=" + url.QueryEscape(id))
	if err != nil {
		return nil, cacheEntry{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, cacheEntry{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, cacheEntry{}, err
	}

	var feed arxivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, cacheEntry{}, err
	}
	if len(feed.Entries) == 0 {
		return nil, cacheEntry{}, fmt.Errorf("no entry found for %s", id)
	}

	ax := feed.Entries[0]
	updated := e
	raw := cacheEntry{Fields: make(map[string]string)}
	var corrections []string

	title := strings.TrimSpace(strings.ReplaceAll(ax.Title, "\n", " "))
	if title != "" {
		// Strip non-escaped braces (non-configurable preprocessing) before caching.
		raw.Fields["title"] = stripNonEscapedBraces(title)
		if applyField(&updated, "title", raw.Fields["title"]) {
			corrections = append(corrections, "title")
		}
	}
	if len(ax.Authors) > 0 {
		raw.Fields["author"] = formatArxivAuthors(ax.Authors)
		if applyField(&updated, "author", raw.Fields["author"]) {
			corrections = append(corrections, "author")
		}
	}
	if len(ax.Published) >= 4 {
		raw.Fields["year"] = ax.Published[:4]
		if applyField(&updated, "year", raw.Fields["year"]) {
			corrections = append(corrections, "year")
		}
	}
	// eprint is known from the query parameter; store it so the cache is complete.
	raw.Fields["eprint"] = id
	if applyField(&updated, "eprint", id) {
		corrections = append(corrections, "eprint")
	}
	if pc := ax.PrimaryCategory.Term; pc != "" {
		raw.Fields["primaryclass"] = pc
		if applyField(&updated, "primaryclass", pc) {
			corrections = append(corrections, "primaryclass")
		}
	}

	if len(corrections) > 0 {
		return &updated, raw, nil
	}
	return nil, raw, nil
}

func formatArxivAuthors(authors []arxivAuthor) string {
	parts := make([]string, 0, len(authors))
	for _, a := range authors {
		name := strings.TrimSpace(a.Name)
		if name != "" {
			parts = append(parts, reverseArxivName(name))
		}
	}
	return strings.Join(parts, " and ")
}

// reverseArxivName converts "First Last" → "Last, First".
// Names already in "Last, First" form are returned unchanged.
func reverseArxivName(name string) string {
	if strings.Contains(name, ",") {
		return name
	}
	parts := strings.Fields(name)
	if len(parts) < 2 {
		return name
	}
	return parts[len(parts)-1] + ", " + strings.Join(parts[:len(parts)-1], " ")
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
