package bib

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type arxivFeed struct {
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	Title           string        `xml:"title"`
	Authors         []arxivAuthor `xml:"author"`
	Published       string        `xml:"published"`
	DOI             string        `xml:"http://arxiv.org/schemas/atom doi"`
	PrimaryCategory struct {
		Term string `xml:"term,attr"`
	} `xml:"http://arxiv.org/schemas/atom primary_category"`
}

type arxivAuthor struct {
	Name string `xml:"name"`
}

func queryArxiv(e Entry, id string, log Logger) (*Entry, cacheEntry, string, error) {
	log = logOrNop(log)
	log.Progress(e.Key, "fetching metadata from arXiv...")
	apiURL := "https://export.arxiv.org/api/query?id_list=" + url.QueryEscape(id)
	resp, err := doWithRetry(func() (*http.Response, error) {
		return httpClient.Get(apiURL)
	}, log, e.Key)
	if err != nil {
		if isRetryableError(err) {
			return nil, cacheEntry{}, "", fmt.Errorf("arXiv request timed out")
		}
		return nil, cacheEntry{}, "", fmt.Errorf("arXiv query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, cacheEntry{}, "", friendlyHTTPError(resp.StatusCode, "arXiv")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, cacheEntry{}, "", err
	}

	var feed arxivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, cacheEntry{}, "", err
	}
	if len(feed.Entries) == 0 {
		return nil, cacheEntry{}, "", fmt.Errorf("no entry found for %s: %w", id, errNotFound)
	}

	ax := feed.Entries[0]
	doi := strings.TrimSpace(ax.DOI)
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
		return &updated, raw, doi, nil
	}
	return nil, raw, doi, nil
}

func formatArxivAuthors(authors []arxivAuthor) string {
	parts := make([]string, 0, len(authors))
	for _, a := range authors {
		name := normalizeAllCapsName(strings.TrimSpace(a.Name))
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
