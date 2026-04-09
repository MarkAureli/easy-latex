package bib

import (
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

// cache maps citation keys to their validation source
// ("crossref", "arxiv", or "no-id").
type cache map[string]string

func loadCache(auxDir string) cache {
	data, err := os.ReadFile(filepath.Join(auxDir, "bib_cache.json"))
	if err != nil {
		return make(cache)
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil {
		return make(cache)
	}
	return c
}

func saveCache(auxDir string, c cache) {
	data, _ := json.MarshalIndent(c, "", "  ")
	_ = os.WriteFile(filepath.Join(auxDir, "bib_cache.json"), data, 0644)
}

// ProcessBibFiles formats and validates every registered .bib file.
func ProcessBibFiles(bibFiles []string, auxDir string, abbreviateJournals, braceTitles, ieeeFormat bool, maxAuthors int, abbreviateFirstName bool) error {
	if len(bibFiles) == 0 {
		return nil
	}
	c := loadCache(auxDir)
	cacheChanged := false

	for _, path := range bibFiles {
		changed, err := processBibFile(path, auxDir, c, abbreviateJournals, braceTitles, ieeeFormat, maxAuthors, abbreviateFirstName)
		if err != nil {
			return err
		}
		if changed {
			cacheChanged = true
		}
	}

	if cacheChanged {
		saveCache(auxDir, c)
	}
	return nil
}

func processBibFile(path, auxDir string, c cache, abbreviateJournals, braceTitles, ieeeFormat bool, maxAuthors int, abbreviateFirstName bool) (cacheChanged bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("cannot read %s: %w", path, err)
	}

	items := ParseFile(string(data))

	assignCanonicalKeys(items)

	// pendingCache collects (itemIndex, source) for entries validated this run.
	// Written to the cache after the second assignCanonicalKeys so the cache
	// key matches the final canonical key.
	type pendingEntry struct {
		itemIdx int
		source  string
	}
	var pending []pendingEntry

	for i, item := range items {
		if !item.IsEntry {
			continue
		}
		e := item.Entry

		if _, seen := c[e.Key]; !seen {
			corrected, source, warn := validateEntry(e, abbreviateJournals)
			if warn != "" {
				fmt.Printf("[bib] %s: %s\n", e.Key, warn)
			}
			if corrected != nil {
				e = *corrected
			}
			if source != "" {
				pending = append(pending, pendingEntry{i, source})
				cacheChanged = true
			}
		}

		normalizeEntryFields(&e)

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
		items[i].Entry = e
	}

	// Re-assign keys: Crossref/arXiv may have populated author, year, or title
	// for entries that previously fell back to their original key.
	assignCanonicalKeys(items)

	// Flush pending cache entries under the final canonical keys.
	for _, p := range pending {
		c[items[p.itemIdx].Entry.Key] = p.source
	}

	formatted := renderItems(items)
	if formatted != string(data) {
		if err := os.WriteFile(path, []byte(formatted), 0644); err != nil {
			return cacheChanged, fmt.Errorf("cannot write %s: %w", path, err)
		}
	}
	return cacheChanged, nil
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
func normalizeEntryFields(e *Entry) {
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
	// Derive url from doi when the type supports both and url is absent.
	// Not applied to arXiv @misc entries, which are identified by eprint.
	if !isArxiv && allowed["url"] && FieldValue(*e, "url") == "" {
		if doi := FieldValue(*e, "doi"); doi != "" {
			SetField(e, "url", "{https://doi.org/"+doi+"}")
		}
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
// corrected entry (nil if nothing changed), the source used, and an optional
// warning.
func validateEntry(e Entry, abbreviateJournals bool) (corrected *Entry, source, warning string) {
	if doi := findDOI(e); doi != "" {
		result, err := queryCrossref(e, doi, abbreviateJournals)
		if err != nil {
			return nil, "", fmt.Sprintf("Crossref query failed: %v", err)
		}
		return result, "crossref", ""
	}

	if id := findArxivID(e); id != "" {
		result, err := queryArxiv(e, id)
		if err != nil {
			return nil, "", fmt.Sprintf("arXiv query failed: %v", err)
		}
		return result, "arxiv", ""
	}

	if doiIsMandatory(e.Type) {
		return nil, "no-id", "no DOI or arXiv ID — skipping validation"
	}
	return nil, "no-id", ""
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

func queryCrossref(e Entry, doi string, abbreviateJournals bool) (*Entry, error) {
	req, err := http.NewRequest("GET", "https://api.crossref.org/works/"+url.PathEscape(doi), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "easy-latex/0.1 (https://github.com/MarkAureli/easy-latex)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var cr crossrefResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, err
	}
	if cr.Status != "ok" {
		return nil, fmt.Errorf("status: %s", cr.Status)
	}

	m := cr.Message
	updated := e
	var corrections []string

	if len(m.Title) > 0 {
		if applyField(&updated, "title", m.Title[0]) {
			corrections = append(corrections, "title")
		}
	}
	if len(m.Author) > 0 {
		if applyField(&updated, "author", formatCrossrefAuthors(m.Author)) {
			corrections = append(corrections, "author")
		}
	}
	if len(m.ContainerTitle) > 0 {
		journal := m.ContainerTitle[0]
		if abbreviateJournals {
			journal = AbbreviateISO4(journal)
		}
		if applyField(&updated, "journal", journal) {
			corrections = append(corrections, "journal")
		}
	}
	if len(m.Published.DateParts) > 0 && len(m.Published.DateParts[0]) > 0 {
		if applyField(&updated, "year", fmt.Sprintf("%d", m.Published.DateParts[0][0])) {
			corrections = append(corrections, "year")
		}
	}
	if m.Volume != "" {
		if applyField(&updated, "volume", m.Volume) {
			corrections = append(corrections, "volume")
		}
	}
	if m.Issue != "" {
		if applyField(&updated, "number", m.Issue) {
			corrections = append(corrections, "number")
		}
	}
	if m.Page != "" {
		if applyField(&updated, "pages", m.Page) {
			corrections = append(corrections, "pages")
		}
	}
	if m.DOI != "" {
		if applyField(&updated, "doi", strings.ToLower(m.DOI)) {
			corrections = append(corrections, "doi")
		}
	}

	if len(corrections) > 0 {
		fmt.Printf("[bib] %s: corrected %s\n", e.Key, strings.Join(corrections, ", "))
		return &updated, nil
	}
	return nil, nil
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
	Title     string        `xml:"title"`
	Authors   []arxivAuthor `xml:"author"`
	Published string        `xml:"published"`
}

type arxivAuthor struct {
	Name string `xml:"name"`
}

func queryArxiv(e Entry, id string) (*Entry, error) {
	resp, err := httpClient.Get("https://export.arxiv.org/api/query?id_list=" + url.QueryEscape(id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var feed arxivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}
	if len(feed.Entries) == 0 {
		return nil, fmt.Errorf("no entry found for %s", id)
	}

	ax := feed.Entries[0]
	updated := e
	var corrections []string

	title := strings.TrimSpace(strings.ReplaceAll(ax.Title, "\n", " "))
	if title != "" {
		if applyField(&updated, "title", title) {
			corrections = append(corrections, "title")
		}
	}
	if len(ax.Authors) > 0 {
		if applyField(&updated, "author", formatArxivAuthors(ax.Authors)) {
			corrections = append(corrections, "author")
		}
	}
	if len(ax.Published) >= 4 {
		if applyField(&updated, "year", ax.Published[:4]) {
			corrections = append(corrections, "year")
		}
	}

	if len(corrections) > 0 {
		fmt.Printf("[bib] %s: corrected %s\n", e.Key, strings.Join(corrections, ", "))
		return &updated, nil
	}
	return nil, nil
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
