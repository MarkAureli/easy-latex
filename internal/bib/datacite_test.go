package bib

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// doiServer serves Crossref (/works/…) as 404 and DataCite (/dois/…) from
// dc (doi → attributes JSON); unknown DataCite DOIs are 404. arXiv queries
// (/api/query) return arxivFeed.
func doiServer(t *testing.T, dc map[string]map[string]any, arxivFeed []byte) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/dois/"):
			doi := strings.TrimPrefix(r.URL.Path, "/dois/")
			attrs, ok := dc[doi]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			b, _ := json.Marshal(map[string]any{"data": map[string]any{"attributes": attrs}})
			w.Write(b)
		case strings.HasPrefix(r.URL.Path, "/api/query"):
			w.Write(arxivFeed)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	t.Cleanup(func() { httpClient = orig })
}

var drakeAttrs = map[string]any{
	"doi":             "10.13140/2.1.3578.9122",
	"creators":        []map[string]any{{"name": "Drake, John H", "nameType": "Personal", "givenName": "John H", "familyName": "Drake"}},
	"titles":          []map[string]any{{"title": "A Subtitle", "titleType": "Subtitle"}, {"title": "Benchmark  instances"}},
	"publisher":       "Unpublished",
	"publicationYear": 2015,
}

func TestFormatDataciteCreators(t *testing.T) {
	got := formatDataciteCreators([]dataciteCreator{
		{Name: "Doe, Jane", NameType: "Personal", GivenName: "Jane", FamilyName: "Doe"},
		{Name: "SMITH, JOHN", NameType: "Personal", GivenName: "JOHN", FamilyName: "SMITH"},
		{Name: "Geoscience Australia", NameType: "Organizational"},
		{Name: "Raka Jovanovic"},
		{Name: ":unav"},
		{Name: "(:unkn)", NameType: "Organizational"},
		{Name: "Plato", NameType: "Personal", FamilyName: "Plato"},
	})
	want := "Doe, Jane and Smith, John and {Geoscience Australia} and Raka Jovanovic and Plato"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestIsDataciteUnknown(t *testing.T) {
	for _, s := range []string{":unav", "(:unkn)", "Unpublished", " unknown ", ""} {
		if !isDataciteUnknown(s) {
			t.Errorf("%q should be unknown", s)
		}
	}
	for _, s := range []string{"Mendeley", "Zenodo", "Uni: Bern"} {
		if isDataciteUnknown(s) {
			t.Errorf("%q should be known", s)
		}
	}
}

func TestValidateEntry_CrossrefMiss_DataCiteMisc(t *testing.T) {
	doiServer(t, map[string]map[string]any{"10.13140/2.1.3578.9122": drakeAttrs}, nil)
	e := Entry{Type: "article", Key: "k", Fields: []Field{
		{Name: "doi", Value: "{10.13140/2.1.3578.9122}"},
		{Name: "journal", Value: "{Bogus}"},
	}}
	got, raw, source, warn := validateEntry(e, false, nil)
	if source != "datacite" || warn != "" {
		t.Fatalf("source = %q, warn = %q", source, warn)
	}
	if got == nil || got.Type != "misc" {
		t.Fatalf("got = %+v, want @misc", got)
	}
	want := map[string]string{
		"author": "Drake, John H",
		"title":  "Benchmark instances",
		"year":   "2015",
		"doi":    "10.13140/2.1.3578.9122",
	}
	for k, v := range want {
		if raw.Fields[k] != v {
			t.Errorf("raw[%s] = %q, want %q", k, raw.Fields[k], v)
		}
	}
	if _, ok := raw.Fields["publisher"]; ok {
		t.Errorf("placeholder publisher kept: %q", raw.Fields["publisher"])
	}
	ce := buildCacheEntry(*got, raw, source, "")
	if _, ok := ce.Fields["journal"]; ok {
		t.Error("journal must be dropped for @misc")
	}
}

func TestValidateEntry_CrossrefAndDataCiteMiss_InvalidID(t *testing.T) {
	doiServer(t, nil, nil)
	e := Entry{Type: "misc", Key: "k", Fields: []Field{{Name: "doi", Value: "{10.1000/nope}"}}}
	_, _, source, warn := validateEntry(e, false, nil)
	if source != "invalid-id" || !strings.Contains(warn, "DataCite") {
		t.Errorf("source = %q, warn = %q", source, warn)
	}
}

func TestAddEntryFromID_DataCite(t *testing.T) {
	doiServer(t, map[string]map[string]any{"10.13140/2.1.3578.9122": drakeAttrs}, nil)
	t.Setenv("EL_GLOBAL_BIB", filepath.Join(t.TempDir(), "bib.json"))
	key, isNew, err := AddEntryFromID("10.13140/2.1.3578.9122", nil)
	if err != nil || !isNew {
		t.Fatalf("key=%q isNew=%v err=%v", key, isNew, err)
	}
	if e := loadCache()[key]; e.Source != "datacite" || e.Type != "misc" {
		t.Errorf("entry = %+v", e)
	}
	if _, _, err := AddEntryFromID("10.1000/nope", nil); err == nil || !strings.Contains(err.Error(), "Crossref or DataCite") {
		t.Errorf("err = %v", err)
	}
}

func TestArxivDOI_Detection(t *testing.T) {
	if got := arxivIDFromDOI("10.48550/arXiv.2301.07041"); got != "2301.07041" {
		t.Errorf("arxivIDFromDOI = %q", got)
	}
	for _, e := range []Entry{
		{Fields: []Field{{Name: "doi", Value: "{10.48550/ARXIV.2301.07041}"}}},
		{Fields: []Field{{Name: "url", Value: "{https://doi.org/10.48550/arXiv.2301.07041}"}}},
	} {
		if findDOI(e) != "" {
			t.Errorf("findDOI(%v) should ignore arXiv DOI", e.Fields)
		}
		if findArxivID(e) != "2301.07041" {
			t.Errorf("findArxivID(%v) = %q", e.Fields, findArxivID(e))
		}
	}
}

func TestValidateEntry_ArxivDOI_BecomesArxivMisc(t *testing.T) {
	doiServer(t, nil, makeArxivXML("Some Preprint", "Jane Doe", "2023-01-17T00:00:00Z"))
	e := Entry{Type: "article", Key: "k", Fields: []Field{{Name: "doi", Value: "{10.48550/arXiv.2301.07041}"}}}
	got, raw, source, warn := validateEntry(e, false, nil)
	if source != "arxiv" || warn != "" || got == nil {
		t.Fatalf("source = %q, warn = %q, got = %+v", source, warn, got)
	}
	ce := buildCacheEntry(*got, raw, source, "")
	if ce.Type != "misc" || ce.Fields["eprint"] != "2301.07041" || ce.Fields["archiveprefix"] != "arXiv" {
		t.Errorf("cache entry = %+v", ce)
	}
	if _, ok := ce.Fields["doi"]; ok {
		t.Error("arXiv DOI should not be stored")
	}
}

func TestAddEntryFromID_ArxivDOI_UsesArxivPath(t *testing.T) {
	doiServer(t, nil, makeArxivXML("Some Preprint", "Jane Doe", "2023-01-17T00:00:00Z"))
	t.Setenv("EL_GLOBAL_BIB", filepath.Join(t.TempDir(), "bib.json"))
	key, isNew, err := AddEntryFromID("https://doi.org/10.48550/arXiv.2301.07041", nil)
	if err != nil || !isNew {
		t.Fatalf("key=%q isNew=%v err=%v", key, isNew, err)
	}
	if e := loadCache()[key]; e.Source != "arxiv" || e.Fields["eprint"] != "2301.07041" {
		t.Errorf("entry = %+v", e)
	}
	k2, isNew2, err := AddEntryFromID("2301.07041", nil)
	if err != nil || isNew2 || k2 != key {
		t.Errorf("dedup by eprint failed: key=%q isNew=%v err=%v", k2, isNew2, err)
	}
}
