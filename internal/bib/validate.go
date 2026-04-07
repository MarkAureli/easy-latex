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
func ProcessBibFiles(bibFiles []string, auxDir string) error {
	if len(bibFiles) == 0 {
		return nil
	}
	c := loadCache(auxDir)
	cacheChanged := false

	for _, path := range bibFiles {
		changed, err := processBibFile(path, auxDir, c)
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

func processBibFile(path, auxDir string, c cache) (cacheChanged bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("cannot read %s: %w", path, err)
	}

	items := ParseFile(string(data))

	assignCanonicalKeys(items)

	for i, item := range items {
		if !item.IsEntry {
			continue
		}
		e := item.Entry

		if _, seen := c[e.Key]; !seen {
			corrected, source, warn := validateEntry(e)
			if warn != "" {
				fmt.Printf("[bib] %s: %s\n", e.Key, warn)
			}
			if corrected != nil {
				e = *corrected
			}
			if source != "" {
				c[e.Key] = source
				cacheChanged = true
			}
		}

		normalizeArticleFields(&e)

		if warn := warnMissingFields(e); warn != "" {
			fmt.Printf("[bib] %s: %s\n", e.Key, warn)
		}

		ensureArticleOptionalFields(&e)

		// Sort fields after validation so any newly added fields are ordered too.
		e.Fields = sortedFields(e.Type, e.Fields)
		items[i].Entry = e
	}

	formatted := renderItems(items)
	if formatted != string(data) {
		if err := os.WriteFile(path, []byte(formatted), 0644); err != nil {
			return cacheChanged, fmt.Errorf("cannot write %s: %w", path, err)
		}
	}
	return cacheChanged, nil
}

// articleMandatory lists fields that must be present (non-blank) in every
// @article entry. volume, number, and pages are intentionally omitted because
// they are legitimately absent for some articles.
var articleMandatory = []string{"author", "title", "journal", "year", "doi", "url"}

// warnMissingFields returns a warning string if any mandatory fields are absent
// from the entry, or an empty string if everything is present.
func warnMissingFields(e Entry) string {
	if e.Type != "article" {
		return ""
	}
	var missing []string
	for _, name := range articleMandatory {
		if FieldValue(e, name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return "missing mandatory fields: " + strings.Join(missing, ", ")
	}
	return ""
}

// articleAllowedFields is the complete set of fields kept in an @article entry.
// Every other field is dropped during processing.
var articleAllowedFields = map[string]bool{
	"author": true, "title": true, "journal": true, "year": true,
	"volume": true, "number": true, "pages": true, "doi": true, "url": true,
}

// normalizeArticleFields drops non-allowed fields from @article entries.
// "issue" is treated as a synonym for "number": if "number" is absent, "issue"
// is renamed to "number"; otherwise "issue" is simply dropped.
func normalizeArticleFields(e *Entry) {
	if e.Type != "article" {
		return
	}
	if FieldValue(*e, "number") == "" {
		if issue := FieldValue(*e, "issue"); issue != "" {
			SetField(e, "number", "{"+issue+"}")
		}
	}
	filtered := make([]Field, 0, len(e.Fields))
	for _, f := range e.Fields {
		if f.Name != "issue" && articleAllowedFields[f.Name] {
			filtered = append(filtered, f)
		}
	}
	e.Fields = filtered

	if FieldValue(*e, "url") == "" {
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
func validateEntry(e Entry) (corrected *Entry, source, warning string) {
	if doi := findDOI(e); doi != "" {
		result, err := queryCrossref(e, doi)
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

	return nil, "no-id", "no DOI or arXiv ID — skipping validation"
}

func findDOI(e Entry) string {
	if doi := FieldValue(e, "doi"); doi != "" {
		return doi
	}
	u := FieldValue(e, "url")
	if idx := strings.Index(u, "doi.org/"); idx >= 0 {
		return u[idx+8:]
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

func queryCrossref(e Entry, doi string) (*Entry, error) {
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
		if applyField(&updated, "journal", m.ContainerTitle[0]) {
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
