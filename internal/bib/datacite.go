package bib

import (
	"cmp"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// reArxivDOI matches arXiv's DataCite DOIs (10.48550/arXiv.<id>) and
// captures the arXiv identifier.
var reArxivDOI = regexp.MustCompile(`(?i)^10\.48550/arxiv\.(.+)$`)

// arxivIDFromDOI returns the arXiv id encoded in an arXiv DOI, or "".
func arxivIDFromDOI(doi string) string {
	if m := reArxivDOI.FindStringSubmatch(strings.TrimSpace(doi)); m != nil {
		return strings.ToLower(m[1])
	}
	return ""
}

type dataciteCreator struct {
	Name       string `json:"name"`
	NameType   string `json:"nameType"`
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
}

type dataciteResponse struct {
	Data struct {
		Attributes struct {
			DOI      string            `json:"doi"`
			Creators []dataciteCreator `json:"creators"`
			Titles   []struct {
				Title     string `json:"title"`
				TitleType string `json:"titleType"`
			} `json:"titles"`
			// Publisher is a plain string in the default serialization.
			Publisher       json.RawMessage `json:"publisher"`
			PublicationYear json.RawMessage `json:"publicationYear"`
		} `json:"attributes"`
	} `json:"data"`
}

// isDataciteUnknown reports whether s is a DataCite "unknown value" code
// (":unav", "(:unkn)", …) or a common placeholder such as "Unpublished".
func isDataciteUnknown(s string) bool {
	v := strings.ToLower(strings.Trim(strings.TrimSpace(s), "()[]"))
	if strings.HasPrefix(v, ":") {
		return true
	}
	switch v {
	case "", "unpublished", "unknown", "n/a", "na", "none", "not available", "anonymous":
		return true
	}
	return false
}

// formatDataciteCreators formats creators as BibTeX authors: personal names
// with a family name become "Family, Given" (all-caps normalised, as for
// Crossref), organisations are braced, untyped names are passed unchanged.
// Unknown-value creators (":unav", …) are skipped.
func formatDataciteCreators(creators []dataciteCreator) string {
	clean := func(s string) string { return strings.Join(strings.Fields(html.UnescapeString(s)), " ") }
	parts := make([]string, 0, len(creators))
	for _, c := range creators {
		name := clean(c.Name)
		family := normalizeAllCapsName(clean(c.FamilyName))
		given := normalizeAllCapsName(clean(c.GivenName))
		switch {
		case c.NameType == "Organizational":
			if !isDataciteUnknown(name) {
				parts = append(parts, "{"+name+"}")
			}
		case family != "" && given != "":
			parts = append(parts, family+", "+given)
		case family != "":
			parts = append(parts, family)
		case !isDataciteUnknown(name):
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, " and ")
}

// dataciteString decodes a JSON string or number into a string.
func dataciteString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	var obj struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.Name
	}
	return ""
}

// queryDataCite fetches DataCite metadata for doi. DataCite entries are
// always @misc (author, year, title, publisher, doi; url derived from doi):
// DataCite's type hints and container metadata are too sparse to build
// richer types reliably. Returns the corrected entry (nil if unchanged) and
// the raw fields for caching.
func queryDataCite(e Entry, doi string, log Logger) (*Entry, cacheEntry, error) {
	log = logOrNop(log)
	log.Progress(e.Key, "fetching metadata from DataCite...")
	req, err := http.NewRequest("GET", "https://api.datacite.org/dois/"+url.PathEscape(doi), nil)
	if err != nil {
		return nil, cacheEntry{}, err
	}
	req.Header.Set("User-Agent", "easy-latex/"+Version+" (https://github.com/MarkAureli/easy-latex)")
	req.Header.Set("Accept", "application/vnd.api+json")

	resp, err := doWithRetry(func() (*http.Response, error) {
		return httpClient.Do(req)
	}, log, e.Key)
	if err != nil {
		if isRetryableError(err) {
			return nil, cacheEntry{}, fmt.Errorf("DataCite request timed out")
		}
		return nil, cacheEntry{}, fmt.Errorf("DataCite query failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, cacheEntry{}, friendlyHTTPError(resp.StatusCode, "DataCite")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, cacheEntry{}, err
	}
	var dr dataciteResponse
	if err := json.Unmarshal(body, &dr); err != nil {
		return nil, cacheEntry{}, err
	}
	a := dr.Data.Attributes

	fields := map[string]string{
		"author": formatDataciteCreators(a.Creators),
		"doi":    strings.ToLower(cmp.Or(a.DOI, doi)),
	}
	// Main title = first title without a titleType (Subtitle, AlternativeTitle, …).
	for _, t := range a.Titles {
		if t.TitleType == "" {
			fields["title"] = cleanCrossrefTitle(stripNonEscapedBraces(html.UnescapeString(t.Title)))
			break
		}
	}
	if y := dataciteString(a.PublicationYear); len(y) >= 4 {
		fields["year"] = y[:4]
	}
	if p := strings.Join(strings.Fields(html.UnescapeString(dataciteString(a.Publisher))), " "); !isDataciteUnknown(p) {
		fields["publisher"] = p
	}
	for k, v := range fields {
		if v == "" {
			delete(fields, k)
		}
	}

	updated := e
	updated.Type = "misc"
	changed := e.Type != "misc"
	for k, v := range fields {
		if applyField(&updated, k, v) {
			changed = true
		}
	}
	raw := cacheEntry{Fields: fields}
	if changed {
		return &updated, raw, nil
	}
	return nil, raw, nil
}
