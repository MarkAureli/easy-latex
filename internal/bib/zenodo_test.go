package bib

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zrec builds a minimal InvenioRDM record JSON for tests.
type zrec struct {
	doi, concept, rtype, title, date, publisher string
	creators                                    [][2]string // {type, name}
	custom                                      map[string]any
}

func (z zrec) json() []byte {
	creators := []map[string]any{}
	for _, c := range z.creators {
		creators = append(creators, map[string]any{"person_or_org": map[string]any{"type": c[0], "name": c[1]}})
	}
	rec := map[string]any{
		"pids":   map[string]any{"doi": map[string]any{"identifier": z.doi}},
		"parent": map[string]any{"pids": map[string]any{"doi": map[string]any{"identifier": z.concept}}},
		"metadata": map[string]any{
			"title":            z.title,
			"publication_date": z.date,
			"publisher":        z.publisher,
			"resource_type":    map[string]any{"id": z.rtype},
			"creators":         creators,
		},
		"custom_fields": z.custom,
	}
	b, _ := json.Marshal(rec)
	return b
}

// zenodoServer serves records by id; ids absent from recs return 404.
// Requests for a key in redirects return the record of the mapped id
// (mimicking Zenodo's concept-id → latest-version redirect).
func zenodoServer(t *testing.T, recs map[string]zrec, redirects map[string]string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/vnd.inveniordm.v1+json" {
			t.Errorf("Accept = %q", got)
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/records/")
		if to, ok := redirects[id]; ok {
			id = to
		}
		rec, ok := recs[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(rec.json())
	}))
	t.Cleanup(srv.Close)
	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	t.Cleanup(func() { httpClient = orig })
}

var softwareRec = zrec{
	doi: "10.5281/zenodo.200", concept: "10.5281/zenodo.100", rtype: "software",
	title: "  yolov5:\n v7.0 ", date: "2022-11-22", publisher: "Zenodo",
	creators: [][2]string{{"personal", "Glenn Jocher"}, {"personal", "Doe, Jane"}, {"organizational", "Ultralytics Team"}},
}

func TestZenodoRecordID(t *testing.T) {
	cases := map[string]string{
		"10.5281/zenodo.7347926": "7347926",
		"10.5281/ZENODO.42":      "42",
		"10.5281/zenodo.abc":     "",
		"10.1103/PhysRev.47.777": "",
		"10.5282/zenodo.1":       "",
	}
	for in, want := range cases {
		if got := zenodoRecordID(in); got != want {
			t.Errorf("zenodoRecordID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQueryZenodo_SoftwareMiscWithConceptDOI(t *testing.T) {
	zenodoServer(t, map[string]zrec{"200": softwareRec}, nil)
	e := Entry{Type: "article", Key: "k", Fields: []Field{{Name: "doi", Value: "{10.5281/zenodo.200}"}}}
	got, raw, typ, err := queryZenodo(e, "200", nil)
	if err != nil {
		t.Fatal(err)
	}
	if typ != "misc" || got == nil || got.Type != "misc" {
		t.Fatalf("type = %q / %+v, want misc", typ, got)
	}
	want := map[string]string{
		"title":     "yolov5: v7.0",
		"author":    "Glenn Jocher and Doe, Jane and {Ultralytics Team}",
		"year":      "2022",
		"doi":       "10.5281/zenodo.100",
		"publisher": "Zenodo",
	}
	for k, v := range want {
		if raw.Fields[k] != v {
			t.Errorf("raw[%s] = %q, want %q", k, raw.Fields[k], v)
		}
	}
	if _, ok := raw.Fields["version"]; ok {
		t.Error("version must not be stored")
	}
}

func TestQueryZenodo_TypeUpgradeAndFallback(t *testing.T) {
	base := zrec{concept: "10.5281/zenodo.1", title: "T", date: "2020", publisher: "Pub",
		creators: [][2]string{{"personal", "Doe, Jane"}}}
	with := func(rtype string, custom map[string]any) zrec {
		r := base
		r.doi, r.rtype, r.custom = "10.5281/zenodo.2", rtype, custom
		return r
	}
	cases := []struct {
		name     string
		rec      zrec
		wantType string
		wantKey  string
		wantVal  string
	}{
		{"article", with("publication-article", map[string]any{"journal:journal": map[string]any{"title": "J. Foo", "volume": "3", "issue": "2", "pages": "5-5"}}), "article", "pages", "5"},
		{"article no journal", with("publication-article", nil), "misc", "publisher", "Pub"},
		{"phd", with("publication-dissertation", map[string]any{"thesis:thesis": map[string]any{"university": "Uni"}}), "phdthesis", "school", "Uni"},
		{"masters", with("publication-dissertation", map[string]any{"thesis:thesis": map[string]any{"university": "Uni", "type": "Masters"}}), "mastersthesis", "school", "Uni"},
		{"thesis no uni", with("publication-dissertation", nil), "misc", "title", "T"},
		{"book", with("publication-book", map[string]any{"imprint:imprint": map[string]any{"place": "Bern"}}), "book", "address", "Bern"},
		{"chapter", with("publication-section", map[string]any{"imprint:imprint": map[string]any{"title": "Big Book", "pages": "1-9"}}), "incollection", "booktitle", "Big Book"},
		{"chapter no booktitle", with("publication-section", nil), "misc", "title", "T"},
		{"conf meeting", with("publication-conferencepaper", map[string]any{"meeting:meeting": map[string]any{"title": "Conf"}}), "inproceedings", "booktitle", "Conf"},
		{"conf imprint wins", with("publication-conferencepaper", map[string]any{"meeting:meeting": map[string]any{"title": "Conf"}, "imprint:imprint": map[string]any{"title": "Proc"}}), "inproceedings", "booktitle", "Proc"},
		{"conf none", with("publication-conferencepaper", nil), "misc", "title", "T"},
		{"dataset", with("dataset", nil), "misc", "publisher", "Pub"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			zenodoServer(t, map[string]zrec{"2": tc.rec}, nil)
			_, raw, typ, err := queryZenodo(Entry{Type: "misc", Key: "k"}, "2", nil)
			if err != nil {
				t.Fatal(err)
			}
			if typ != tc.wantType {
				t.Errorf("type = %q, want %q", typ, tc.wantType)
			}
			if raw.Fields[tc.wantKey] != tc.wantVal {
				t.Errorf("raw[%s] = %q, want %q", tc.wantKey, raw.Fields[tc.wantKey], tc.wantVal)
			}
			for k := range raw.Fields {
				if !entrySpecs[typ].allowed[k] {
					t.Errorf("field %q not allowed for @%s", k, typ)
				}
			}
		})
	}
}

func TestQueryZenodo_BookWithoutPublisherFallsBack(t *testing.T) {
	r := zrec{doi: "10.5281/zenodo.2", concept: "10.5281/zenodo.1", rtype: "publication-book",
		title: "T", date: "2020", creators: [][2]string{{"personal", "Doe, Jane"}}}
	zenodoServer(t, map[string]zrec{"2": r}, nil)
	_, _, typ, err := queryZenodo(Entry{Type: "misc", Key: "k"}, "2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if typ != "misc" {
		t.Errorf("type = %q, want misc", typ)
	}
}

func TestValidateEntry_Zenodo404_InvalidID(t *testing.T) {
	zenodoServer(t, nil, nil)
	e := Entry{Type: "misc", Key: "k", Fields: []Field{{Name: "doi", Value: "{10.5281/zenodo.9}"}}}
	_, _, source, warn := resolveForTest(e)
	if source != "invalid-id" {
		t.Errorf("source = %q, want invalid-id", source)
	}
	if !strings.Contains(warn, "Zenodo") {
		t.Errorf("warn = %q", warn)
	}
}

func TestValidateEntry_ZenodoSource(t *testing.T) {
	zenodoServer(t, map[string]zrec{"200": softwareRec}, nil)
	e := Entry{Type: "misc", Key: "k", Fields: []Field{{Name: "doi", Value: "{10.5281/zenodo.200}"}}}
	got, _, source, warn := resolveForTest(e)
	if warn != "" || source != "zenodo" {
		t.Fatalf("source = %q, warn = %q", source, warn)
	}
	if FieldValue(*got, "doi") != "10.5281/zenodo.100" {
		t.Errorf("doi = %q, want concept DOI", FieldValue(*got, "doi"))
	}
}

func TestAddEntryFromID_ZenodoVersionDOI_StoresConceptAndDedups(t *testing.T) {
	v2 := softwareRec
	v2.doi = "10.5281/zenodo.300"
	zenodoServer(t, map[string]zrec{"200": softwareRec, "300": v2}, map[string]string{"100": "300"})
	t.Setenv("EL_GLOBAL_BIB", filepath.Join(t.TempDir(), "bib.json"))

	key, isNew, err := AddEntryFromID("https://doi.org/10.5281/zenodo.200", nil)
	if err != nil || !isNew {
		t.Fatalf("first add: key=%q isNew=%v err=%v", key, isNew, err)
	}
	entry := loadCache()[key]
	if entry.Source != "zenodo" || entry.Type != "misc" || entry.Fields["doi"] != "10.5281/zenodo.100" {
		t.Errorf("entry = %+v", entry)
	}
	if entry.Fields["url"] != "" {
		t.Errorf("url stored raw = %q, want derived at write time", entry.Fields["url"])
	}
	for _, id := range []string{"10.5281/zenodo.300", "10.5281/zenodo.100"} {
		k2, isNew2, err := AddEntryFromID(id, nil)
		if err != nil || isNew2 || k2 != key {
			t.Errorf("add %s: key=%q isNew=%v err=%v, want %q existing", id, k2, isNew2, err, key)
		}
	}
	if n := len(loadCache()); n != 1 {
		t.Errorf("cache size = %d, want 1", n)
	}
}

func TestAllocateCacheEntries_ZenodoVersionsMergeIntoConcept(t *testing.T) {
	v2 := softwareRec
	v2.doi = "10.5281/zenodo.300"
	zenodoServer(t, map[string]zrec{"200": softwareRec, "300": v2}, nil)
	dir := t.TempDir()
	t.Setenv("EL_GLOBAL_BIB", filepath.Join(dir, "bib.json"))
	bibContent := "@misc{A,\n  title = {yolov5: v7.0},\n  author = {Glenn Jocher},\n  year = {2022},\n  doi = {10.5281/zenodo.200},\n}\n\n" +
		"@misc{B,\n  title = {yolov5: v7.0},\n  author = {Glenn Jocher},\n  year = {2022},\n  doi = {10.5281/zenodo.300},\n}\n"
	path := filepath.Join(dir, "a.bib")
	if err := os.WriteFile(path, []byte(bibContent), 0644); err != nil {
		t.Fatal(err)
	}
	added, renames, err := AllocateCacheEntries([]string{path}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
	c := loadCache()
	if len(c) != 1 {
		t.Fatalf("cache = %+v, want 1 entry", c)
	}
	var key string
	for k := range c {
		key = k
	}
	if strings.HasSuffix(key, "a") || strings.HasSuffix(key, "b") {
		t.Errorf("key %q got a collision suffix", key)
	}
	if renames["A"] != key || renames["B"] != key {
		t.Errorf("renames = %v, want A,B → %q", renames, key)
	}

	// A second file citing another version merges into the cached entry.
	path2 := filepath.Join(dir, "b.bib")
	if err := os.WriteFile(path2, []byte("@misc{C,\n  title = {Other},\n  doi = {10.5281/zenodo.300},\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	added, renames, err = AllocateCacheEntries([]string{path2}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 || renames["C"] != key || len(loadCache()) != 1 {
		t.Errorf("second run: added=%d renames=%v cache=%d", added, renames, len(loadCache()))
	}
}
