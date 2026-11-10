package bib

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// reZenodoDOI matches Zenodo-minted DOIs and captures the record id.
var reZenodoDOI = regexp.MustCompile(`(?i)^10\.5281/zenodo\.(\d+)$`)

// zenodoRecordID returns the Zenodo record id encoded in doi, or "" if doi is
// not a Zenodo DOI.
func zenodoRecordID(doi string) string {
	if m := reZenodoDOI.FindStringSubmatch(strings.TrimSpace(doi)); m != nil {
		return m[1]
	}
	return ""
}

type zenodoPID struct {
	Identifier string `json:"identifier"`
}

// zenodoRecord is the subset of Zenodo's native InvenioRDM record
// serialization (Accept: application/vnd.inveniordm.v1+json) used by el.
// The legacy serialization omits publisher and imprint title, so it is not used.
type zenodoRecord struct {
	PIDs struct {
		DOI zenodoPID `json:"doi"`
	} `json:"pids"`
	Parent struct {
		PIDs struct {
			DOI zenodoPID `json:"doi"`
		} `json:"pids"`
	} `json:"parent"`
	Metadata struct {
		Title           string `json:"title"`
		PublicationDate string `json:"publication_date"`
		Publisher       string `json:"publisher"`
		ResourceType    struct {
			ID string `json:"id"`
		} `json:"resource_type"`
		Creators []struct {
			PersonOrOrg struct {
				Type string `json:"type"`
				Name string `json:"name"`
			} `json:"person_or_org"`
		} `json:"creators"`
	} `json:"metadata"`
	CustomFields struct {
		Journal struct {
			Title  string `json:"title"`
			Volume string `json:"volume"`
			Issue  string `json:"issue"`
			Pages  string `json:"pages"`
		} `json:"journal:journal"`
		Imprint struct {
			Title string `json:"title"`
			Pages string `json:"pages"`
			Place string `json:"place"`
		} `json:"imprint:imprint"`
		Meeting struct {
			Title string `json:"title"`
		} `json:"meeting:meeting"`
		Thesis struct {
			University string `json:"university"`
			Type       string `json:"type"`
		} `json:"thesis:thesis"`
	} `json:"custom_fields"`
}

// zenodoCandidateType maps a Zenodo resource type id to the richest BibTeX
// type it may become. Everything else is @misc.
func zenodoCandidateType(id string, thesisType string) string {
	switch id {
	case "publication-article":
		return "article"
	case "publication-conferencepaper":
		return "inproceedings"
	case "publication-section":
		return "incollection"
	case "publication-book":
		return "book"
	case "publication-dissertation":
		if strings.Contains(strings.ToLower(thesisType), "master") {
			return "mastersthesis"
		}
		return "phdthesis"
	default:
		return "misc"
	}
}

// formatZenodoCreators joins creator names unchanged with " and ". Zenodo
// names are "Family, Given" or free-form ("Glenn Jocher", GitHub handles);
// organisations are wrapped in braces so BibTeX treats them as one entity.
func formatZenodoCreators(r *zenodoRecord) string {
	parts := make([]string, 0, len(r.Metadata.Creators))
	for _, c := range r.Metadata.Creators {
		name := strings.Join(strings.Fields(c.PersonOrOrg.Name), " ")
		if name == "" {
			continue
		}
		if c.PersonOrOrg.Type == "organizational" {
			name = "{" + name + "}"
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, " and ")
}

// zenodoFields maps a Zenodo record onto BibTeX fields for the given type.
// doi is the DOI to store (the concept DOI when available).
func zenodoFields(r *zenodoRecord, bibType, doi string) map[string]string {
	m := r.Metadata
	cf := r.CustomFields
	clean := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	f := map[string]string{
		"title":  stripNonEscapedBraces(clean(m.Title)),
		"author": formatZenodoCreators(r),
		"doi":    strings.ToLower(doi),
	}
	if len(m.PublicationDate) >= 4 {
		f["year"] = m.PublicationDate[:4]
	}
	switch bibType {
	case "article":
		f["journal"] = clean(cf.Journal.Title)
		f["volume"] = clean(cf.Journal.Volume)
		f["number"] = clean(cf.Journal.Issue)
		f["pages"] = collapsePageRange(clean(cf.Journal.Pages))
	case "inproceedings":
		f["booktitle"] = clean(cmp.Or(cf.Imprint.Title, cf.Meeting.Title))
		f["pages"] = collapsePageRange(clean(cf.Imprint.Pages))
	case "incollection":
		f["booktitle"] = clean(cf.Imprint.Title)
		f["publisher"] = clean(m.Publisher)
		f["address"] = clean(cf.Imprint.Place)
		f["pages"] = collapsePageRange(clean(cf.Imprint.Pages))
	case "book":
		f["publisher"] = clean(m.Publisher)
		f["address"] = clean(cf.Imprint.Place)
	case "phdthesis", "mastersthesis":
		f["school"] = clean(cf.Thesis.University)
	case "misc":
		f["publisher"] = clean(m.Publisher)
	}
	for k, v := range f {
		if v == "" {
			delete(f, k)
		}
	}
	return f
}

// zenodoEntry builds a BibTeX entry and its field map from a Zenodo record.
// The richest candidate type is used only if Zenodo supplies all its
// mandatory fields; otherwise the entry falls back to @misc.
func zenodoEntry(r *zenodoRecord, doi string) (string, map[string]string) {
	bibType := zenodoCandidateType(r.Metadata.ResourceType.ID, r.CustomFields.Thesis.Type)
	fields := zenodoFields(r, bibType, doi)
	if bibType != "misc" {
		probe := Entry{Type: bibType}
		for k, v := range fields {
			probe.Fields = append(probe.Fields, Field{Name: k, Value: "{" + v + "}"})
		}
		normalizeEntryFields(&probe, false)
		if len(missingMandatoryFields(probe)) > 0 {
			bibType = "misc"
			fields = zenodoFields(r, bibType, doi)
		}
	}
	return bibType, fields
}

// queryZenodo fetches Zenodo record recid and returns the corrected entry
// (nil if nothing changed), the raw fields for caching and the BibTeX type.
// The stored DOI is the record's concept DOI (all versions) when available,
// so a version DOI is replaced by its concept DOI.
func queryZenodo(e Entry, recid string, log Logger) (*Entry, cacheEntry, string, error) {
	log = logOrNop(log)
	log.Progress(e.Key, "fetching metadata from Zenodo...")
	req, err := http.NewRequest("GET", "https://zenodo.org/api/records/"+recid, nil)
	if err != nil {
		return nil, cacheEntry{}, "", err
	}
	req.Header.Set("User-Agent", "easy-latex/"+Version+" (https://github.com/MarkAureli/easy-latex)")
	req.Header.Set("Accept", "application/vnd.inveniordm.v1+json")

	resp, err := doWithRetry(func() (*http.Response, error) {
		return httpClient.Do(req)
	}, log, e.Key)
	if err != nil {
		if isRetryableError(err) {
			return nil, cacheEntry{}, "", fmt.Errorf("Zenodo request timed out")
		}
		return nil, cacheEntry{}, "", fmt.Errorf("Zenodo query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusGone {
		// Deleted record: treat as not found.
		return nil, cacheEntry{}, "", friendlyHTTPError(http.StatusNotFound, "Zenodo")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, cacheEntry{}, "", friendlyHTTPError(resp.StatusCode, "Zenodo")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, cacheEntry{}, "", err
	}
	var rec zenodoRecord
	if err := json.Unmarshal(body, &rec); err != nil {
		return nil, cacheEntry{}, "", err
	}

	inputDOI := "10.5281/zenodo." + recid
	doi := cmp.Or(rec.Parent.PIDs.DOI.Identifier, cmp.Or(rec.PIDs.DOI.Identifier, inputDOI))
	if !strings.EqualFold(doi, inputDOI) && strings.EqualFold(rec.PIDs.DOI.Identifier, inputDOI) {
		log.Info(e.Key, fmt.Sprintf("%s is a Zenodo version DOI; using concept DOI %s", inputDOI, strings.ToLower(doi)))
	}

	bibType, fields := zenodoEntry(&rec, doi)
	updated := e
	updated.Type = bibType
	raw := cacheEntry{Fields: fields}
	changed := e.Type != bibType
	for k, v := range fields {
		if applyField(&updated, k, v) {
			changed = true
		}
	}
	if changed {
		return &updated, raw, bibType, nil
	}
	return nil, raw, bibType, nil
}
