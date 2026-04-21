package bib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

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
		Type   string `json:"type"`
	} `json:"message"`
}

// mapCrossrefType maps a Crossref work type to a BibTeX entry type.
// Returns empty string if the type is unknown.
func mapCrossrefType(crType string) string {
	switch crType {
	case "journal-article":
		return "article"
	case "proceedings-article":
		return "inproceedings"
	case "book-chapter":
		return "incollection"
	case "book", "monograph", "edited-book", "reference-book":
		return "book"
	case "report", "report-component":
		return "techreport"
	case "dissertation":
		return "phdthesis"
	default:
		return ""
	}
}

// containerTitleField returns the BibTeX field name that Crossref's
// container-title should map to for the given entry type.
func containerTitleField(bibType string) string {
	switch bibType {
	case "inproceedings", "conference", "incollection":
		return "booktitle"
	default:
		return "journal"
	}
}

func queryCrossref(e Entry, doi string, log Logger) (*Entry, cacheEntry, string, error) {
	log = logOrNop(log)
	log.Progress(e.Key, "fetching metadata from Crossref...")
	req, err := http.NewRequest("GET", "https://api.crossref.org/works/"+url.PathEscape(doi), nil)
	if err != nil {
		return nil, cacheEntry{}, "", err
	}
	req.Header.Set("User-Agent", "easy-latex/"+Version+" (https://github.com/MarkAureli/easy-latex)")

	resp, err := doWithRetry(func() (*http.Response, error) {
		return httpClient.Do(req)
	}, log, e.Key)
	if err != nil {
		if isRetryableError(err) {
			return nil, cacheEntry{}, "", fmt.Errorf("Crossref request timed out")
		}
		return nil, cacheEntry{}, "", fmt.Errorf("Crossref query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, cacheEntry{}, "", friendlyHTTPError(resp.StatusCode, "Crossref")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, cacheEntry{}, "", err
	}

	var cr crossrefResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, cacheEntry{}, "", err
	}
	if cr.Status != "ok" {
		return nil, cacheEntry{}, "", fmt.Errorf("status: %s", cr.Status)
	}

	m := cr.Message
	bibType := mapCrossrefType(m.Type)
	updated := e
	raw := cacheEntry{Fields: make(map[string]string)}
	var corrections []string

	if len(m.Title) > 0 {
		// Strip non-escaped braces here (non-configurable preprocessing) so the
		// cached title is already in its final pre-transformation state.
		// cleanCrossrefTitle runs after brace stripping to convert MathML and
		// Crossref face markup (e.g. <i>, <sub>) to LaTeX equivalents.
		raw.Fields["title"] = cleanCrossrefTitle(stripNonEscapedBraces(m.Title[0]))
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
		// Map container-title to journal or booktitle based on entry type.
		ctField := containerTitleField(bibType)
		raw.Fields[ctField] = m.ContainerTitle[0]
		if applyField(&updated, ctField, raw.Fields[ctField]) {
			corrections = append(corrections, ctField)
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
		return &updated, raw, bibType, nil
	}
	return nil, raw, bibType, nil
}

func formatCrossrefAuthors(authors []crossrefAuthor) string {
	parts := make([]string, 0, len(authors))
	for _, a := range authors {
		family := normalizeAllCapsName(a.Family)
		given := normalizeAllCapsName(a.Given)
		switch {
		case family != "" && given != "":
			parts = append(parts, family+", "+given)
		case family != "":
			parts = append(parts, family)
		default:
			parts = append(parts, given)
		}
	}
	return strings.Join(parts, " and ")
}
