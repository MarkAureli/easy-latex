package bib

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

// ── findDOI ───────────────────────────────────────────────────────────────────

func TestFindDOI_ExplicitField(t *testing.T) {
	e := Entry{Fields: []Field{{Name: "doi", Value: "{10.1000/xyz}"}}}
	if got := findDOI(e); got != "10.1000/xyz" {
		t.Errorf("got %q, want %q", got, "10.1000/xyz")
	}
}

func TestFindDOI_FromURL(t *testing.T) {
	e := Entry{Fields: []Field{{Name: "url", Value: "{https://doi.org/10.1000/xyz}"}}}
	if got := findDOI(e); got != "10.1000/xyz" {
		t.Errorf("got %q, want %q", got, "10.1000/xyz")
	}
}

func TestFindDOI_NoField(t *testing.T) {
	e := Entry{Fields: []Field{{Name: "url", Value: "{https://example.com}"}}}
	if got := findDOI(e); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ── findArxivID ───────────────────────────────────────────────────────────────

func TestFindArxivID_EprintWithArchivePrefix(t *testing.T) {
	e := Entry{Fields: []Field{
		{Name: "eprint", Value: "{2301.00001}"},
		{Name: "archiveprefix", Value: "{arXiv}"},
	}}
	if got := findArxivID(e); got != "2301.00001" {
		t.Errorf("got %q, want %q", got, "2301.00001")
	}
}

func TestFindArxivID_EprintWithEprintType(t *testing.T) {
	e := Entry{Fields: []Field{
		{Name: "eprint", Value: "{2301.00001}"},
		{Name: "eprinttype", Value: "{arXiv}"},
	}}
	if got := findArxivID(e); got != "2301.00001" {
		t.Errorf("got %q, want %q", got, "2301.00001")
	}
}

func TestFindArxivID_FromURL(t *testing.T) {
	e := Entry{Fields: []Field{
		{Name: "url", Value: "{https://arxiv.org/abs/2301.00001}"},
	}}
	if got := findArxivID(e); got != "2301.00001" {
		t.Errorf("got %q, want %q", got, "2301.00001")
	}
}

func TestFindArxivID_EprintWithoutPrefix(t *testing.T) {
	e := Entry{Fields: []Field{
		{Name: "eprint", Value: "{2301.00001}"},
	}}
	if got := findArxivID(e); got != "" {
		t.Errorf("expected empty without archiveprefix, got %q", got)
	}
}

func TestFindArxivID_None(t *testing.T) {
	e := Entry{Fields: []Field{{Name: "title", Value: "{X}"}}}
	if got := findArxivID(e); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ── formatCrossrefAuthors ─────────────────────────────────────────────────────

func TestFormatCrossrefAuthors_FamilyAndGiven(t *testing.T) {
	authors := []crossrefAuthor{{Family: "Smith", Given: "John"}, {Family: "Doe", Given: "Jane"}}
	want := "Smith, John and Doe, Jane"
	if got := formatCrossrefAuthors(authors); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCrossrefAuthors_FamilyOnly(t *testing.T) {
	authors := []crossrefAuthor{{Family: "Smith"}}
	if got := formatCrossrefAuthors(authors); got != "Smith" {
		t.Errorf("got %q, want %q", got, "Smith")
	}
}

func TestFormatCrossrefAuthors_GivenOnly(t *testing.T) {
	authors := []crossrefAuthor{{Given: "Anonymous"}}
	if got := formatCrossrefAuthors(authors); got != "Anonymous" {
		t.Errorf("got %q, want %q", got, "Anonymous")
	}
}

func TestFormatCrossrefAuthors_GroupAuthor(t *testing.T) {
	authors := []crossrefAuthor{
		{Name: "Google Quantum AI"},
		{Family: "Acharya", Given: "Rajeev"},
	}
	want := "{Google Quantum AI}"
	if got := formatCrossrefAuthors(authors); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatCrossrefAuthors_GroupAuthorOnly(t *testing.T) {
	authors := []crossrefAuthor{{Name: "PsiQuantum team"}}
	want := "{PsiQuantum team}"
	if got := formatCrossrefAuthors(authors); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ── reverseArxivName ──────────────────────────────────────────────────────────

func TestReverseArxivName_ReversesFirstLast(t *testing.T) {
	if got := reverseArxivName("John Smith"); got != "Smith, John" {
		t.Errorf("got %q, want %q", got, "Smith, John")
	}
}

func TestReverseArxivName_ThreePartName(t *testing.T) {
	if got := reverseArxivName("John A. Smith"); got != "Smith, John A." {
		t.Errorf("got %q, want %q", got, "Smith, John A.")
	}
}

func TestReverseArxivName_AlreadyReversed(t *testing.T) {
	if got := reverseArxivName("Smith, John"); got != "Smith, John" {
		t.Errorf("got %q, want %q", got, "Smith, John")
	}
}

func TestReverseArxivName_SingleName(t *testing.T) {
	if got := reverseArxivName("Madonna"); got != "Madonna" {
		t.Errorf("got %q, want %q", got, "Madonna")
	}
}

// ── normalizeFieldValue ───────────────────────────────────────────────────────

func TestNormalizeFieldValue(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "hello world"},
		{"  hello   world  ", "hello world"},
		{"UPPER", "upper"},
	}
	for _, c := range cases {
		if got := normalizeFieldValue(c.in); got != c.want {
			t.Errorf("normalizeFieldValue(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── applyField ────────────────────────────────────────────────────────────────

func TestApplyField_UpdatesWhenDifferent(t *testing.T) {
	e := Entry{Fields: []Field{{Name: "title", Value: "{Old Title}"}}}
	changed := applyField(&e, "title", "New Title")
	if !changed {
		t.Fatal("expected changed=true")
	}
	if got := FieldValue(e, "title"); got != "New Title" {
		t.Errorf("got %q, want %q", got, "New Title")
	}
}

func TestApplyField_NoChangeWhenSame(t *testing.T) {
	e := Entry{Fields: []Field{{Name: "title", Value: "{Same Title}"}}}
	changed := applyField(&e, "title", "Same Title")
	if changed {
		t.Fatal("expected changed=false for identical value")
	}
}

func TestApplyField_CaseInsensitiveNoop(t *testing.T) {
	e := Entry{Fields: []Field{{Name: "title", Value: "{Hello World}"}}}
	changed := applyField(&e, "title", "hello world")
	if changed {
		t.Fatal("expected changed=false for case-only difference")
	}
}

func TestApplyField_AddsNewField(t *testing.T) {
	e := Entry{}
	changed := applyField(&e, "year", "2023")
	if !changed {
		t.Fatal("expected changed=true when adding new field")
	}
	if got := FieldValue(e, "year"); got != "2023" {
		t.Errorf("got %q, want %q", got, "2023")
	}
}

// ── queryCrossref (HTTP mock) ─────────────────────────────────────────────────

func makeCrossrefJSON(title, family, given, container, year, volume, issue, page, doi string) []byte {
	return makeCrossrefJSONWithType(title, family, given, container, year, volume, issue, page, doi, "journal-article")
}

func makeCrossrefJSONWithType(title, family, given, container, year, volume, issue, page, doi, crType string) []byte {
	type author struct {
		Family string `json:"family"`
		Given  string `json:"given"`
	}
	type published struct {
		DateParts [][]int `json:"date-parts"`
	}
	type message struct {
		Title          []string  `json:"title"`
		Author         []author  `json:"author"`
		ContainerTitle []string  `json:"container-title"`
		Published      published `json:"published"`
		Volume         string    `json:"volume"`
		Issue          string    `json:"issue"`
		Page           string    `json:"page"`
		DOI            string    `json:"DOI"`
		Type           string    `json:"type"`
	}
	type response struct {
		Status  string  `json:"status"`
		Message message `json:"message"`
	}
	yr := 2023
	if y, err := strconv.Atoi(year); err == nil {
		yr = y
	}
	r := response{
		Status: "ok",
		Message: message{
			Title:          []string{title},
			Author:         []author{{Family: family, Given: given}},
			ContainerTitle: []string{container},
			Published:      published{DateParts: [][]int{{yr}}},
			Volume:         volume,
			Issue:          issue,
			Page:           page,
			DOI:            doi,
			Type:           crType,
		},
	}
	b, _ := json.Marshal(r)
	return b
}

func TestQueryCrossref_CorrectsMismatchedFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeCrossrefJSON(
			"Correct Title", "Smith", "John", "Nature",
			"2023", "42", "3", "100--110", "10.1000/xyz",
		))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Key: "Smith2023",
		Fields: []Field{
			{Name: "title", Value: "{Wrong Title}"},
			{Name: "doi", Value: "{10.1000/xyz}"},
		},
	}
	result, raw, crType, err := queryCrossref(e, "10.1000/xyz", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected correction, got nil")
	}
	if crType != "article" {
		t.Errorf("crType = %q, want article", crType)
	}
	if got := FieldValue(*result, "title"); got != "Correct Title" {
		t.Errorf("title = %q, want %q", got, "Correct Title")
	}
	// raw.Fields must capture all API-provided fields.
	wantFields := map[string]string{
		"title":   "Correct Title",
		"author":  "Smith, John",
		"journal": "Nature",
		"year":    "2023",
		"volume":  "42",
		"number":  "3",
		"pages":   "100--110",
		"doi":     "10.1000/xyz",
	}
	for k, want := range wantFields {
		if got := raw.Fields[k]; got != want {
			t.Errorf("raw.Fields[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestQueryCrossref_NoChangeWhenFieldsMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeCrossrefJSON(
			"My Title", "Smith", "John", "Nature",
			"2023", "1", "2", "3--4", "10.1/x",
		))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Key: "X",
		Fields: []Field{
			{Name: "title", Value: "{My Title}"},
			{Name: "author", Value: "{Smith, John}"},
			{Name: "journal", Value: "{Nature}"},
			{Name: "year", Value: "{2023}"},
			{Name: "volume", Value: "{1}"},
			{Name: "number", Value: "{2}"},
			{Name: "pages", Value: "{3--4}"},
			{Name: "doi", Value: "{10.1/x}"},
		},
	}
	result, _, crType, err := queryCrossref(e, "10.1/x", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil (no change), got corrected entry")
	}
	if crType != "article" {
		t.Errorf("crType = %q, want article", crType)
	}
}

func TestQueryCrossref_ProceedingsType_MapsToInproceedings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeCrossrefJSONWithType(
			"A Conference Paper", "Doe", "Jane", "Proc. ICML",
			"2023", "", "", "1-10", "10.1/conf", "proceedings-article",
		))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Key:    "Doe2023",
		Fields: []Field{{Name: "doi", Value: "{10.1/conf}"}},
	}
	result, raw, crType, err := queryCrossref(e, "10.1/conf", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if crType != "inproceedings" {
		t.Errorf("crType = %q, want inproceedings", crType)
	}
	if result == nil {
		t.Fatal("expected correction, got nil")
	}
	// container-title should map to booktitle, not journal.
	if got := raw.Fields["booktitle"]; got != "Proc. ICML" {
		t.Errorf("raw.Fields[\"booktitle\"] = %q, want %q", got, "Proc. ICML")
	}
	if _, ok := raw.Fields["journal"]; ok {
		t.Errorf("raw.Fields has \"journal\" key, want only \"booktitle\" for proceedings")
	}
	if got := FieldValue(*result, "booktitle"); got != "Proc. ICML" {
		t.Errorf("booktitle = %q, want %q", got, "Proc. ICML")
	}
}

func TestQueryCrossref_UnknownType_ReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeCrossrefJSONWithType(
			"Something", "Doe", "Jane", "",
			"2023", "", "", "", "10.1/x", "dataset",
		))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Key:    "Doe2023",
		Fields: []Field{{Name: "doi", Value: "{10.1/x}"}},
	}
	_, _, crType, err := queryCrossref(e, "10.1/x", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if crType != "" {
		t.Errorf("crType = %q, want empty for unknown type", crType)
	}
}

func TestValidateEntry_DOI_SetsCrossrefType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeCrossrefJSONWithType(
			"A Conference Paper", "Doe", "Jane", "Proc. ICML",
			"2023", "", "", "1-10", "10.1/conf", "proceedings-article",
		))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Type: "article",
		Key:  "Doe2023",
		Fields: []Field{
			{Name: "title", Value: "{Wrong Title}"},
			{Name: "doi", Value: "{10.1/conf}"},
		},
	}
	result, _, source, warn := validateEntry(e, false, nil)
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	if source != "crossref" {
		t.Errorf("source = %q, want crossref", source)
	}
	if result == nil {
		t.Fatal("expected corrected entry, got nil")
	}
	if result.Type != "inproceedings" {
		t.Errorf("type = %q, want inproceedings", result.Type)
	}
}

func TestAddEntryFromID_DOI_SetsCrossrefType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeCrossrefJSONWithType(
			"A Conference Paper", "Doe", "Jane", "Proc. NeurIPS",
			"2023", "", "", "1-10", "10.1/conf", "proceedings-article",
		))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	dir := t.TempDir()
	key, _, err := AddEntryFromID("10.1/conf", dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := loadCache(dir)
	entry, ok := c[key]
	if !ok {
		t.Fatalf("key %q not in cache", key)
	}
	if entry.Type != "inproceedings" {
		t.Errorf("type = %q, want inproceedings", entry.Type)
	}
	if entry.Source != "crossref" {
		t.Errorf("source = %q, want crossref", entry.Source)
	}
}

// ── queryArxiv (HTTP mock) ────────────────────────────────────────────────────

func makeArxivXML(title, authors, published string) []byte {
	type author struct {
		Name string `xml:"name"`
	}
	type entry struct {
		Title     string   `xml:"title"`
		Authors   []author `xml:"author"`
		Published string   `xml:"published"`
	}
	type feed struct {
		XMLName struct{} `xml:"feed"`
		Entries []entry  `xml:"entry"`
	}
	var authorList []author
	for _, a := range splitAuthors(authors) {
		authorList = append(authorList, author{Name: a})
	}
	f := feed{Entries: []entry{{Title: title, Authors: authorList, Published: published}}}
	b, _ := xml.Marshal(f)
	return b
}

// makeArxivXMLWithCategory produces an arXiv Atom feed with a primary_category element.
func makeArxivXMLWithCategory(title, authors, published, primaryClass string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:arxiv="http://arxiv.org/schemas/atom">
  <entry>
    <title>` + title + `</title>
    <author><name>` + authors + `</name></author>
    <published>` + published + `</published>
    <arxiv:primary_category term="` + primaryClass + `" scheme="http://arxiv.org/schemas/atom"/>
  </entry>
</feed>`)
}

func splitAuthors(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

// makeArxivXMLWithDOI produces an arXiv Atom feed with a <arxiv:doi> element.
func makeArxivXMLWithDOI(title, authors, published, doi string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:arxiv="http://arxiv.org/schemas/atom">
  <entry>
    <title>` + title + `</title>
    <author><name>` + authors + `</name></author>
    <published>` + published + `</published>
    <arxiv:doi>` + doi + `</arxiv:doi>
  </entry>
</feed>`)
}

func TestQueryArxiv_CorrectsMismatchedTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write(makeArxivXML("Correct Title", "Smith, John", "2023-01-15T00:00:00Z"))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Key:    "Smith2023",
		Fields: []Field{{Name: "title", Value: "{Wrong Title}"}},
	}
	result, raw, doi, err := queryArxiv(e, "2301.00001", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected correction, got nil")
	}
	if doi != "" {
		t.Errorf("doi = %q, want empty", doi)
	}
	if got := FieldValue(*result, "title"); got != "Correct Title" {
		t.Errorf("title = %q, want %q", got, "Correct Title")
	}
	// raw.Fields must capture all API-provided fields.
	if got := raw.Fields["title"]; got != "Correct Title" {
		t.Errorf("raw.Fields[\"title\"] = %q, want %q", got, "Correct Title")
	}
	if got := raw.Fields["author"]; got != "Smith, John" {
		t.Errorf("raw.Fields[\"author\"] = %q, want %q", got, "Smith, John")
	}
	if got := raw.Fields["year"]; got != "2023" {
		t.Errorf("raw.Fields[\"year\"] = %q, want %q", got, "2023")
	}
	if got := raw.Fields["eprint"]; got != "2301.00001" {
		t.Errorf("raw.Fields[\"eprint\"] = %q, want %q", got, "2301.00001")
	}
}

func TestQueryArxiv_ExtractsYearFromPublished(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write(makeArxivXML("A Title", "Doe, Jane", "2019-06-01T00:00:00Z"))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Key:    "Doe2019",
		Fields: []Field{{Name: "title", Value: "{A Title}"}},
	}
	result, _, _, err := queryArxiv(e, "1906.00001", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a correction (year should be added)")
	}
	if got := FieldValue(*result, "year"); got != "2019" {
		t.Errorf("year = %q, want %q", got, "2019")
	}
}

func TestQueryArxiv_ExtractsPrimaryClass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write(makeArxivXMLWithCategory("A Title", "Smith, John", "2023-01-15T00:00:00Z", "cs.LG"))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Key: "Smith2023",
		Fields: []Field{
			{Name: "title", Value: "{A Title}"},
			{Name: "eprint", Value: "{2301.00001}"},
			{Name: "archiveprefix", Value: "{arXiv}"},
		},
	}
	_, raw, _, err := queryArxiv(e, "2301.00001", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := raw.Fields["primaryclass"]; got != "cs.LG" {
		t.Errorf("raw.Fields[\"primaryclass\"] = %q, want %q", got, "cs.LG")
	}
	if got := raw.Fields["eprint"]; got != "2301.00001" {
		t.Errorf("raw.Fields[\"eprint\"] = %q, want %q", got, "2301.00001")
	}
}

func TestQueryArxiv_ExtractsDOI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write(makeArxivXMLWithDOI("A Title", "Smith, John", "2023-01-15T00:00:00Z", "10.1016/j.test.2023.1234"))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Key:    "Smith2023",
		Fields: []Field{{Name: "title", Value: "{Wrong Title}"}},
	}
	_, _, doi, err := queryArxiv(e, "2301.00001", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doi != "10.1016/j.test.2023.1234" {
		t.Errorf("doi = %q, want %q", doi, "10.1016/j.test.2023.1234")
	}
}

func TestQueryArxiv_NoDOI_ReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write(makeArxivXML("A Title", "Smith, John", "2023-01-15T00:00:00Z"))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Key:    "Smith2023",
		Fields: []Field{{Name: "title", Value: "{A Title}"}},
	}
	_, _, doi, err := queryArxiv(e, "2301.00001", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doi != "" {
		t.Errorf("doi = %q, want empty", doi)
	}
}

// ── warnMissingFields ─────────────────────────────────────────────────────────

func TestWarnMissingFields_AllPresent(t *testing.T) {
	e := Entry{
		Type: "article",
		Fields: []Field{
			{Name: "author", Value: "{A}"},
			{Name: "title", Value: "{T}"},
			{Name: "journal", Value: "{J}"},
			{Name: "year", Value: "{2023}"},
			{Name: "doi", Value: "{10.1/x}"},
			{Name: "url", Value: "{https://example.com}"},
		},
	}
	if got := warnMissingFields(e); got != "" {
		t.Errorf("expected empty warning, got %q", got)
	}
}

func TestWarnMissingFields_OptionalFieldsAbsent(t *testing.T) {
	// volume, number, pages are allowed to be absent — no warning expected
	e := Entry{
		Type: "article",
		Fields: []Field{
			{Name: "author", Value: "{A}"},
			{Name: "title", Value: "{T}"},
			{Name: "journal", Value: "{J}"},
			{Name: "year", Value: "{2023}"},
			{Name: "doi", Value: "{10.1/x}"},
			{Name: "url", Value: "{https://example.com}"},
		},
	}
	if got := warnMissingFields(e); got != "" {
		t.Errorf("volume/number/pages should not trigger warning, got %q", got)
	}
}

// ── normalizeEntryFields ──────────────────────────────────────────────────────

func TestNormalizeEntryFields_ArticleDropsUnknownFields(t *testing.T) {
	e := Entry{
		Type: "article",
		Fields: []Field{
			{Name: "author", Value: "{A}"},
			{Name: "note", Value: "{some note}"},
			{Name: "abstract", Value: "{long text}"},
			{Name: "keywords", Value: "{foo, bar}"},
		},
	}
	normalizeEntryFields(&e, false)
	spec := entrySpecs["article"]
	for _, f := range e.Fields {
		if !spec.allowed[f.Name] {
			t.Errorf("unexpected field %q kept after normalization", f.Name)
		}
	}
}

func TestNormalizeEntryFields_ArticleRenamesIssueToNumber(t *testing.T) {
	e := Entry{
		Type:   "article",
		Fields: []Field{{Name: "issue", Value: "{3}"}},
	}
	normalizeEntryFields(&e, false)
	if got := FieldValue(e, "number"); got != "3" {
		t.Errorf("number = %q, want %q", got, "3")
	}
	for _, f := range e.Fields {
		if f.Name == "issue" {
			t.Error("issue field should have been removed")
		}
	}
}

func TestNormalizeEntryFields_ArticleIssueDroppedWhenNumberPresent(t *testing.T) {
	e := Entry{
		Type: "article",
		Fields: []Field{
			{Name: "number", Value: "{5}"},
			{Name: "issue", Value: "{3}"},
		},
	}
	normalizeEntryFields(&e, false)
	if got := FieldValue(e, "number"); got != "5" {
		t.Errorf("number should be unchanged, got %q", got)
	}
	for _, f := range e.Fields {
		if f.Name == "issue" {
			t.Error("issue field should have been dropped")
		}
	}
}

func TestNormalizeEntryFields_ConstructsURLFromDOI(t *testing.T) {
	e := Entry{
		Type:   "article",
		Fields: []Field{{Name: "doi", Value: "{10.1000/xyz}"}},
	}
	normalizeEntryFields(&e, false)
	if got := FieldValue(e, "url"); got != "https://doi.org/10.1000/xyz" {
		t.Errorf("url = %q, want %q", got, "https://doi.org/10.1000/xyz")
	}
}

func TestNormalizeEntryFields_DoesNotOverwriteExistingURL(t *testing.T) {
	e := Entry{
		Type: "article",
		Fields: []Field{
			{Name: "doi", Value: "{10.1000/xyz}"},
			{Name: "url", Value: "{https://example.com}"},
		},
	}
	normalizeEntryFields(&e, false)
	if got := FieldValue(e, "url"); got != "https://example.com" {
		t.Errorf("url overwritten: got %q", got)
	}
}

func TestNormalizeEntryFields_UrlFromDOIOverwritesExistingURL(t *testing.T) {
	e := Entry{
		Type: "article",
		Fields: []Field{
			{Name: "doi", Value: "{10.1000/xyz}"},
			{Name: "url", Value: "{https://example.com}"},
		},
	}
	normalizeEntryFields(&e, true)
	if got := FieldValue(e, "url"); got != "https://doi.org/10.1000/xyz" {
		t.Errorf("url = %q, want %q", got, "https://doi.org/10.1000/xyz")
	}
}

func TestNormalizeEntryFields_UnknownTypeUnchanged(t *testing.T) {
	e := Entry{
		Type:   "unknown",
		Fields: []Field{{Name: "note", Value: "{kept}"}},
	}
	normalizeEntryFields(&e, false)
	if FieldValue(e, "note") != "kept" {
		t.Error("unknown type fields should not be stripped")
	}
}

func TestNormalizeEntryFields_MiscArxivNormalizesArchivePrefix(t *testing.T) {
	e := Entry{
		Type: "misc",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "eprint", Value: "{2301.00001}"},
			{Name: "archiveprefix", Value: "{arxiv}"},
		},
	}
	normalizeEntryFields(&e, false)
	if got := FieldValue(e, "archiveprefix"); got != "arXiv" {
		t.Errorf("archiveprefix = %q, want %q", got, "arXiv")
	}
}

func TestNormalizeEntryFields_MiscArxivSetsArchivePrefixWhenAbsent(t *testing.T) {
	e := Entry{
		Type: "misc",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "eprint", Value: "{2301.00001}"},
			{Name: "eprinttype", Value: "{arxiv}"},
		},
	}
	normalizeEntryFields(&e, false)
	if got := FieldValue(e, "archiveprefix"); got != "arXiv" {
		t.Errorf("archiveprefix = %q, want %q", got, "arXiv")
	}
}

func TestNormalizeEntryFields_BookDropsUnknownFields(t *testing.T) {
	e := Entry{
		Type: "book",
		Fields: []Field{
			{Name: "author", Value: "{A}"},
			{Name: "edition", Value: "{2nd}"},
			{Name: "note", Value: "{some note}"},
		},
	}
	normalizeEntryFields(&e, false)
	spec := entrySpecs["book"]
	for _, f := range e.Fields {
		if !spec.allowed[f.Name] {
			t.Errorf("unexpected field %q kept after book normalization", f.Name)
		}
	}
}

func TestEnsureArticleOptionalFields_AddsBlankPlaceholders(t *testing.T) {
	e := Entry{Type: "article", Fields: []Field{{Name: "title", Value: "{T}"}}}
	ensureArticleOptionalFields(&e)
	for _, name := range []string{"volume", "number", "pages"} {
		if FieldValue(e, name) != "" {
			t.Errorf("%s should be blank placeholder, got %q", name, FieldValue(e, name))
		}
		found := false
		for _, f := range e.Fields {
			if f.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("field %q not added", name)
		}
	}
}

func TestEnsureArticleOptionalFields_DoesNotOverwriteExisting(t *testing.T) {
	e := Entry{
		Type:   "article",
		Fields: []Field{{Name: "pages", Value: "{100--200}"}},
	}
	ensureArticleOptionalFields(&e)
	if got := FieldValue(e, "pages"); got != "100--200" {
		t.Errorf("pages overwritten: got %q, want %q", got, "100--200")
	}
}

func TestEnsureArticleOptionalFields_NonArticleUnchanged(t *testing.T) {
	e := Entry{Type: "book", Fields: []Field{{Name: "title", Value: "{T}"}}}
	before := len(e.Fields)
	ensureArticleOptionalFields(&e)
	if len(e.Fields) != before {
		t.Error("non-article entry should not be modified")
	}
}

func TestWarnMissingFields_MissingDOIAndURL(t *testing.T) {
	e := Entry{
		Type: "article",
		Fields: []Field{
			{Name: "author", Value: "{A}"},
			{Name: "title", Value: "{T}"},
			{Name: "journal", Value: "{J}"},
			{Name: "year", Value: "{2023}"},
		},
	}
	warn := warnMissingFields(e)
	if warn == "" {
		t.Fatal("expected warning for missing doi and url")
	}
	if !containsField(warn, "doi") || !containsField(warn, "url") {
		t.Errorf("warning should mention doi and url, got %q", warn)
	}
}

func TestWarnMissingFields_UnknownTypeIgnored(t *testing.T) {
	e := Entry{Type: "unknown", Fields: []Field{{Name: "title", Value: "{T}"}}}
	if got := warnMissingFields(e); got != "" {
		t.Errorf("unknown entry types should not be checked, got %q", got)
	}
}

func containsField(warn, field string) bool {
	return strings.Contains(warn, field)
}

// ── validateEntry ─────────────────────────────────────────────────────────────

func TestValidateEntry_NoIDWarning_ArticleWarns(t *testing.T) {
	e := Entry{Type: "article", Key: "NoID", Fields: []Field{{Name: "title", Value: "{X}"}}}
	corrected, _, source, warn := validateEntry(e, true, nil)
	if corrected != nil {
		t.Error("expected no correction")
	}
	if source != "no-id" {
		t.Errorf("source = %q, want %q", source, "no-id")
	}
	if warn == "" {
		t.Error("expected a warning for article with no DOI or arXiv ID")
	}
}

func TestValidateEntry_NoIDWarning_BookSuppressed(t *testing.T) {
	e := Entry{Type: "book", Key: "NoID", Fields: []Field{{Name: "title", Value: "{X}"}}}
	_, _, source, warn := validateEntry(e, true, nil)
	if source != "no-id" {
		t.Errorf("source = %q, want %q", source, "no-id")
	}
	if warn != "" {
		t.Errorf("expected no warning for book without doi, got %q", warn)
	}
}

// ── cache re-application ──────────────────────────────────────────────────────

// TestCacheReappliesAllFields verifies that on a second ProcessBibFiles call
// (entry already in cache), all Fields from the cache are written back to the
// bib file, overriding whatever was in the file.
func TestCacheReappliesAllFields(t *testing.T) {
	dir := t.TempDir()
	c := cache{
		"Smith2023StaleTitle": cacheEntry{
			Source: "crossref",
			Type:   "article",
			Fields: map[string]string{
				"author":  "Smith, John",
				"title":   "Correct Title",
				"journal": "Nature",
				"year":    "2023",
				"volume":  "99",
				"number":  "7",
				"pages":   "1--10",
				"doi":     "10.1/x",
			},
		},
	}
	saveCache(dir, c, nil)

	outPath := dir + "/bibliography.bib"
	if err := WriteBibFromCache(outPath, []string{"Smith2023StaleTitle"}, dir, WriteOptions{AbbreviateFirstName: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	items := ParseFile(string(data))
	var entry Entry
	for _, it := range items {
		if it.IsEntry {
			entry = it.Entry
			break
		}
	}
	checks := map[string]string{
		"title":   "Correct Title",
		"journal": "Nature",
		"volume":  "99",
		"number":  "7",
		"pages":   "1--10",
	}
	for field, want := range checks {
		if got := FieldValue(entry, field); got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
}

// TestCacheJournalAbbreviationOnReapply verifies that the full journal name in
// the cache is abbreviated when abbreviateJournals=true, even for cached entries.
func TestCacheJournalAbbreviationOnReapply(t *testing.T) {
	dir := t.TempDir()
	c := cache{
		"Smith2023ATitle": cacheEntry{
			Source: "crossref",
			Type:   "article",
			Fields: map[string]string{
				"author":  "Smith, John",
				"title":   "A Title",
				"journal": "Nature Communications",
				"year":    "2023",
				"doi":     "10.1/x",
			},
		},
	}
	saveCache(dir, c, nil)

	outPath := dir + "/bibliography.bib"
	if err := WriteBibFromCache(outPath, []string{"Smith2023ATitle"}, dir, WriteOptions{AbbreviateJournals: true, AbbreviateFirstName: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	items := ParseFile(string(data))
	var entry Entry
	for _, it := range items {
		if it.IsEntry {
			entry = it.Entry
			break
		}
	}
	got := FieldValue(entry, "journal")
	if got == "Nature Communications" {
		t.Error("journal should have been abbreviated but was not")
	}
	if got == "" {
		t.Error("journal should not be empty")
	}
}

// TestCacheRawURLPreservedOnReapply verifies that a raw url stored in the
// cache is written back and not overridden when urlFromDOI=false.
func TestCacheRawURLPreservedOnReapply(t *testing.T) {
	dir := t.TempDir()
	c := cache{
		"Smith2023ATitle": cacheEntry{
			Source: "crossref",
			Type:   "article",
			Fields: map[string]string{
				"author":  "Smith, John",
				"title":   "A Title",
				"journal": "Nature",
				"year":    "2023",
				"doi":     "10.1/x",
				"url":     "https://custom.example.com/paper",
			},
		},
	}
	saveCache(dir, c, nil)

	outPath := dir + "/bibliography.bib"
	if err := WriteBibFromCache(outPath, []string{"Smith2023ATitle"}, dir, WriteOptions{AbbreviateFirstName: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	items := ParseFile(string(data))
	var entry Entry
	for _, it := range items {
		if it.IsEntry {
			entry = it.Entry
			break
		}
	}
	if got := FieldValue(entry, "url"); got != "https://custom.example.com/paper" {
		t.Errorf("url = %q, want raw cached url", got)
	}
}

// ── brace titles ─────────────────────────────────────────────────────────────

func TestBraceTitles_AppliesDoublebraces(t *testing.T) {
	dir := t.TempDir()
	saveCache(dir, cache{
		"Smith2023Test": cacheEntry{
			Source: "crossref",
			Type:   "article",
			Fields: map[string]string{
				"author":  "Smith, John",
				"title":   "My Test Title",
				"journal": "Nature",
				"year":    "2023",
				"doi":     "10.1/x",
				"url":     "https://doi.org/10.1/x",
			},
		},
	}, nil)

	outPath := dir + "/bibliography.bib"
	if err := WriteBibFromCache(outPath, []string{"Smith2023Test"}, dir, WriteOptions{AbbreviateJournals: true, BraceTitles: true, AbbreviateFirstName: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	items := ParseFile(string(data))
	var entry Entry
	for _, it := range items {
		if it.IsEntry {
			entry = it.Entry
			break
		}
	}
	// Field.Value should be {{My Test Title}} — raw value includes outer delimiters
	raw := ""
	for _, f := range entry.Fields {
		if f.Name == "title" {
			raw = f.Value
			break
		}
	}
	if raw != "{{My Test Title}}" {
		t.Errorf("title raw value = %q, want %q", raw, "{{My Test Title}}")
	}
}

func TestBraceTitles_Idempotent(t *testing.T) {
	dir := t.TempDir()
	// Cache stores the title without braces; WriteBibFromCache applies braceTitles fresh.
	saveCache(dir, cache{
		"Smith2023Test": cacheEntry{
			Source: "crossref",
			Type:   "article",
			Fields: map[string]string{
				"author":  "Smith, John",
				"title":   "My Test Title",
				"journal": "Nature",
				"year":    "2023",
				"doi":     "10.1/x",
				"url":     "https://doi.org/10.1/x",
			},
		},
	}, nil)

	outPath := dir + "/bibliography.bib"
	// First write
	if err := WriteBibFromCache(outPath, []string{"Smith2023Test"}, dir, WriteOptions{AbbreviateJournals: true, BraceTitles: true, AbbreviateFirstName: true}); err != nil {
		t.Fatalf("first write: unexpected error: %v", err)
	}

	data1, _ := os.ReadFile(outPath)
	items := ParseFile(string(data1))
	var entry Entry
	for _, it := range items {
		if it.IsEntry {
			entry = it.Entry
			break
		}
	}
	raw1 := ""
	for _, f := range entry.Fields {
		if f.Name == "title" {
			raw1 = f.Value
			break
		}
	}
	if raw1 != "{{My Test Title}}" {
		t.Errorf("first write: title raw value = %q, want %q", raw1, "{{My Test Title}}")
	}

	// Verify idempotency: read output, parse, re-cache, write again
	c := loadCache(dir)
	var cachedEntry cacheEntry
	for _, ce := range c {
		cachedEntry = ce
		break
	}

	saveCache(dir, cache{"Smith2023Test": cachedEntry}, nil)

	outPath2 := dir + "/bibliography2.bib"
	if err := WriteBibFromCache(outPath2, []string{"Smith2023Test"}, dir, WriteOptions{AbbreviateJournals: true, BraceTitles: true, AbbreviateFirstName: true}); err != nil {
		t.Fatalf("second write: unexpected error: %v", err)
	}

	data2, _ := os.ReadFile(outPath2)
	if string(data1) != string(data2) {
		t.Errorf("output not idempotent: first write differs from second write")
	}
}

func TestBraceTitles_Disabled_LeavesTitle(t *testing.T) {
	dir := t.TempDir()
	saveCache(dir, cache{
		"Smith2023Test": cacheEntry{
			Source: "crossref",
			Type:   "article",
			Fields: map[string]string{
				"author":  "Smith, John",
				"title":   "My Test Title",
				"journal": "Nature",
				"year":    "2023",
				"doi":     "10.1/x",
				"url":     "https://doi.org/10.1/x",
			},
		},
	}, nil)

	outPath := dir + "/bibliography.bib"
	if err := WriteBibFromCache(outPath, []string{"Smith2023Test"}, dir, WriteOptions{AbbreviateJournals: true, AbbreviateFirstName: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	items := ParseFile(string(data))
	var entry Entry
	for _, it := range items {
		if it.IsEntry {
			entry = it.Entry
			break
		}
	}
	raw := ""
	for _, f := range entry.Fields {
		if f.Name == "title" {
			raw = f.Value
			break
		}
	}
	if raw != "{My Test Title}" {
		t.Errorf("title raw value = %q, want %q", raw, "{My Test Title}")
	}
}


// ── transformArxivMiscToUnpublished ───────────────────────────────────────────

func TestTransformArxivMiscToUnpublished_TypeChanged(t *testing.T) {
	e := Entry{
		Type: "misc",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "year", Value: "{2023}"},
			{Name: "title", Value: "{My Paper}"},
			{Name: "eprint", Value: "{2301.00001}"},
			{Name: "archiveprefix", Value: "{arXiv}"},
			{Name: "primaryclass", Value: "{cs.LG}"},
		},
	}
	transformArxivMiscToUnpublished(&e)
	if e.Type != "unpublished" {
		t.Errorf("type = %q, want %q", e.Type, "unpublished")
	}
}

func TestTransformArxivMiscToUnpublished_DropsArxivFields(t *testing.T) {
	e := Entry{
		Type: "misc",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "year", Value: "{2023}"},
			{Name: "title", Value: "{My Paper}"},
			{Name: "eprint", Value: "{2301.00001}"},
			{Name: "archiveprefix", Value: "{arXiv}"},
			{Name: "primaryclass", Value: "{cs.LG}"},
		},
	}
	transformArxivMiscToUnpublished(&e)
	for _, f := range e.Fields {
		if f.Name == "eprint" || f.Name == "archiveprefix" || f.Name == "primaryclass" {
			t.Errorf("field %q should have been dropped", f.Name)
		}
	}
}

func TestTransformArxivMiscToUnpublished_KeepsAuthorYearTitle(t *testing.T) {
	e := Entry{
		Type: "misc",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "year", Value: "{2023}"},
			{Name: "title", Value: "{My Paper}"},
			{Name: "eprint", Value: "{2301.00001}"},
			{Name: "archiveprefix", Value: "{arXiv}"},
		},
	}
	transformArxivMiscToUnpublished(&e)
	for _, name := range []string{"author", "year", "title"} {
		if FieldValue(e, name) == "" {
			t.Errorf("field %q should have been kept", name)
		}
	}
}

func TestTransformArxivMiscToUnpublished_NoteContainsHref(t *testing.T) {
	e := Entry{
		Type: "misc",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "year", Value: "{2023}"},
			{Name: "title", Value: "{My Paper}"},
			{Name: "eprint", Value: "{2301.00001}"},
			{Name: "archiveprefix", Value: "{arXiv}"},
		},
	}
	transformArxivMiscToUnpublished(&e)
	note := FieldValue(e, "note")
	wantURL := "https://arxiv.org/abs/2301.00001"
	wantLabel := "arXiv:2301.00001"
	if !strings.Contains(note, wantURL) {
		t.Errorf("note %q does not contain URL %q", note, wantURL)
	}
	if !strings.Contains(note, wantLabel) {
		t.Errorf("note %q does not contain label %q", note, wantLabel)
	}
	if !strings.Contains(note, `\href`) {
		t.Errorf("note %q does not contain \\href", note)
	}
}

// ── IEEE format (processBibFile integration) ──────────────────────────────────

func TestArxivAsUnpublished_ArxivMiscBecomesUnpublished(t *testing.T) {
	dir := t.TempDir()
	bibContent := `@misc{Smith2023MyPaper,
  author       = {Smith, John},
  year         = {2023},
  title        = {My Paper},
  eprint       = {2301.00001},
  archiveprefix = {arXiv},
  primaryclass = {cs.LG},
}
`
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bibContent), 0644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := AllocateCacheEntries([]string{path}, dir, true, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cache must store the arXiv entry as @misc with eprint preserved.
	keys := LoadCacheKeys(dir)
	if len(keys) == 0 {
		t.Fatal("cache is empty after AllocateCacheEntries")
	}
	c := loadCache(dir)
	for _, k := range keys {
		if e := c[k]; e.Type != "misc" {
			t.Errorf("cache type = %q, want %q (arXiv entries must be cached as @misc)", e.Type, "misc")
		}
		if e := c[k]; e.Fields["eprint"] == "" {
			t.Error("eprint should be preserved in cache")
		}
	}

	// WriteBibFromCache must apply the transform: @misc arXiv -> @unpublished.
	outPath := dir + "/bibliography.bib"
	if err := WriteBibFromCache(outPath, keys, dir, WriteOptions{ArxivAsUnpublished: true}); err != nil {
		t.Fatalf("WriteBibFromCache: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	items := ParseFile(string(data))
	var outEntry Entry
	for _, it := range items {
		if it.IsEntry {
			outEntry = it.Entry
			break
		}
	}
	if outEntry.Type != "unpublished" {
		t.Errorf("bibliography.bib type = %q, want %q", outEntry.Type, "unpublished")
	}
	for _, name := range []string{"eprint", "archiveprefix", "primaryclass"} {
		if FieldValue(outEntry, name) != "" {
			t.Errorf("field %q should have been dropped in bibliography.bib", name)
		}
	}
	note := FieldValue(outEntry, "note")
	if !strings.Contains(note, "2301.00001") {
		t.Errorf("note %q does not reference eprint", note)
	}
	if !strings.Contains(note, `\href`) {
		t.Errorf("note %q does not contain \\href", note)
	}
}

func TestBraceTitles_DoubleBracesTitle(t *testing.T) {
	dir := t.TempDir()
	saveCache(dir, cache{
		"Smith2023Test": cacheEntry{
			Source: "crossref",
			Type:   "article",
			Fields: map[string]string{
				"author":  "Smith, John",
				"title":   "My Test Title",
				"journal": "Nature",
				"year":    "2023",
				"doi":     "10.1/x",
				"url":     "https://doi.org/10.1/x",
			},
		},
	}, nil)

	outPath := dir + "/bibliography.bib"
	// brace_titles=true — should double-brace titles
	if err := WriteBibFromCache(outPath, []string{"Smith2023Test"}, dir, WriteOptions{AbbreviateJournals: true, BraceTitles: true, AbbreviateFirstName: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	items := ParseFile(string(data))
	var entry Entry
	for _, it := range items {
		if it.IsEntry {
			entry = it.Entry
			break
		}
	}
	raw := ""
	for _, f := range entry.Fields {
		if f.Name == "title" {
			raw = f.Value
			break
		}
	}
	if raw != "{{My Test Title}}" {
		t.Errorf("title raw = %q, want %q", raw, "{{My Test Title}}")
	}
}

func TestArxivAsUnpublished_NonArxivMiscUnchanged(t *testing.T) {
	dir := t.TempDir()
	saveCache(dir, cache{
		"Smith2023Software": cacheEntry{
			Source: "no-id",
			Type:   "misc",
			Fields: map[string]string{
				"author": "Smith, John",
				"year":   "2023",
				"title":  "Some Software",
				"url":    "https://example.com",
			},
		},
	}, nil)

	outPath := dir + "/bibliography.bib"
	if err := WriteBibFromCache(outPath, []string{"Smith2023Software"}, dir, WriteOptions{AbbreviateJournals: true, ArxivAsUnpublished: true, AbbreviateFirstName: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(outPath)
	items := ParseFile(string(data))
	var entry Entry
	for _, it := range items {
		if it.IsEntry {
			entry = it.Entry
			break
		}
	}
	if entry.Type != "misc" {
		t.Errorf("type = %q, want %q (non-arXiv misc should stay misc)", entry.Type, "misc")
	}
}

// ── AllocateCacheEntries ──────────────────────────────────────────────────────

func TestAllocateCacheEntries_NoIDEntryAdded(t *testing.T) {
	dir := t.TempDir()
	bibContent := "@misc{SomeKey,\n  author = {Doe, Jane},\n  title = {Some Title},\n  year = {2024},\n}\n"
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bibContent), 0644); err != nil {
		t.Fatal(err)
	}

	added, _, err := AllocateCacheEntries([]string{path}, dir, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
	c := loadCache(dir)
	var found bool
	for _, entry := range c {
		if entry.Source == "no-id" {
			found = true
		}
	}
	if !found {
		t.Error("expected a no-id cache entry, found none")
	}
}

func TestAllocateCacheEntries_NoIDDedup_ByCanonicalKey(t *testing.T) {
	dir := t.TempDir()
	// Pre-populate cache with the canonical key that the bib entry will produce.
	saveCache(dir, cache{"Doe2024SomeTitle": cacheEntry{Source: "no-id", Type: "misc", Fields: map[string]string{"author": "Doe, Jane", "title": "Some Title", "year": "2024"}}}, nil)

	bibContent := "@misc{AnyKey,\n  author = {Doe, Jane},\n  title = {Some Title},\n  year = {2024},\n}\n"
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bibContent), 0644); err != nil {
		t.Fatal(err)
	}

	added, _, err := AllocateCacheEntries([]string{path}, dir, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 0 {
		t.Errorf("added = %d, want 0 (canonical key already in cache)", added)
	}
}

func TestAllocateCacheEntries_DOIDedup_ByDOI(t *testing.T) {
	dir := t.TempDir()
	// Cache already has an entry whose Fields["doi"] matches.
	saveCache(dir, cache{
		"Smith2020Title": cacheEntry{
			Source: "crossref",
			Type:   "article",
			Fields: map[string]string{"doi": "10.1/existing"},
		},
	}, nil)

	bibContent := "@article{AnyKey,\n  author = {Smith, A.},\n  title = {Title},\n  year = {2020},\n  doi = {10.1/existing},\n}\n"
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bibContent), 0644); err != nil {
		t.Fatal(err)
	}

	// No HTTP calls expected; if Crossref is reached the test will hang/fail.
	added, _, err := AllocateCacheEntries([]string{path}, dir, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 0 {
		t.Errorf("added = %d, want 0 (DOI already in cache)", added)
	}
}

func TestAllocateCacheEntries_ArxivDedup_ByEprint(t *testing.T) {
	dir := t.TempDir()
	saveCache(dir, cache{
		"Doe2023Title": cacheEntry{
			Source: "arxiv",
			Type:   "misc",
			Fields: map[string]string{"eprint": "2301.00001"},
		},
	}, nil)

	bibContent := "@misc{AnyKey,\n  author = {Doe, Jane},\n  title = {Title},\n  year = {2023},\n  eprint = {2301.00001},\n  archiveprefix = {arXiv},\n}\n"
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bibContent), 0644); err != nil {
		t.Fatal(err)
	}

	added, _, err := AllocateCacheEntries([]string{path}, dir, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 0 {
		t.Errorf("added = %d, want 0 (arXiv ID already in cache)", added)
	}
}

func TestAllocateCacheEntries_DOIDedup_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	saveCache(dir, cache{
		"Smith2020Title": cacheEntry{
			Source: "crossref",
			Type:   "article",
			Fields: map[string]string{"doi": "10.1/UPPER"},
		},
	}, nil)

	bibContent := "@article{AnyKey,\n  author = {Smith, A.},\n  title = {Title},\n  year = {2020},\n  doi = {10.1/upper},\n}\n"
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bibContent), 0644); err != nil {
		t.Fatal(err)
	}

	added, _, err := AllocateCacheEntries([]string{path}, dir, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 0 {
		t.Errorf("added = %d, want 0 (DOI matched case-insensitively)", added)
	}
}

func TestAllocateCacheEntries_NewDOIEntry_ValidatedAndCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeCrossrefJSON("Correct Title", "Smith", "John", "Nature", "2023", "42", "3", "1--10", "10.1/new"))
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	dir := t.TempDir()
	bibContent := "@article{AnyKey,\n  author = {Smith, John},\n  title = {Wrong Title},\n  year = {2023},\n  doi = {10.1/new},\n}\n"
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bibContent), 0644); err != nil {
		t.Fatal(err)
	}

	added, _, err := AllocateCacheEntries([]string{path}, dir, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}
	c := loadCache(dir)
	var found bool
	for _, entry := range c {
		if entry.Source == "crossref" && entry.Fields["doi"] == "10.1/new" {
			found = true
		}
	}
	if !found {
		t.Error("expected crossref cache entry with doi=10.1/new")
	}
}

// TestValidateEntry_CrossrefHTTP429_Warning verifies that a 429 response from
// Crossref results in a warning string and leaves the entry uncorrected.
func TestValidateEntry_CrossrefHTTP429_Warning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Type: "article",
		Key:  "Smith2024Test",
		Fields: []Field{
			{Name: "author", Value: "{Smith, Jane}"},
			{Name: "year", Value: "{2024}"},
			{Name: "title", Value: "{Some Title}"},
			{Name: "doi", Value: "{10.1000/test}"},
		},
	}
	corrected, _, source, warn := validateEntry(e, false, nil)
	if corrected != nil {
		t.Error("expected no correction on HTTP 429")
	}
	if source != "timeout" {
		t.Errorf("source = %q, want %q", source, "timeout")
	}
	if !strings.Contains(warn, "429") {
		t.Errorf("warning should mention 429, got %q", warn)
	}
}

func TestValidateEntry_Crossref404_InvalidID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Type: "article",
		Key:  "Smith2024Bad",
		Fields: []Field{
			{Name: "author", Value: "{Smith, Jane}"},
			{Name: "year", Value: "{2024}"},
			{Name: "title", Value: "{Some Title}"},
			{Name: "doi", Value: "{10.1000/nonexistent}"},
		},
	}
	corrected, _, source, warn := validateEntry(e, false, nil)
	if corrected != nil {
		t.Error("expected no correction on HTTP 404")
	}
	if source != "invalid-id" {
		t.Errorf("source = %q, want %q", source, "invalid-id")
	}
	if warn == "" {
		t.Error("expected a warning for invalid DOI")
	}
}

func TestAllocateCacheEntries_TimeoutRetry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	dir := t.TempDir()
	bibContent := `@article{Smith2024Test,
  author = {Smith, Jane},
  year   = {2024},
  title  = {Some Title},
  doi    = {10.1000/test},
}
`
	path := dir + "/refs.bib"
	os.WriteFile(path, []byte(bibContent), 0644)

	// Pre-seed cache with a "timeout" entry for this DOI.
	saveCache(dir, cache{"Smith2024SomeTitle": cacheEntry{
		Source: "timeout",
		Type:   "article",
		Fields: map[string]string{
			"author": "Smith, Jane",
			"year":   "2024",
			"title":  "Some Title",
			"doi":    "10.1000/test",
		},
	}}, nil)

	// With retryTimeout=true, should re-validate (calls Crossref).
	calls = 0
	AllocateCacheEntries([]string{path}, dir, true, nil)
	if calls == 0 {
		t.Error("expected timeout entry to be re-validated when retryTimeout=true")
	}

	// With retryTimeout=false, should skip (no Crossref call).
	calls = 0
	AllocateCacheEntries([]string{path}, dir, false, nil)
	if calls != 0 {
		t.Errorf("expected no re-validation when retryTimeout=false, got %d calls", calls)
	}
}

// rebaseTransport redirects all requests to a test server base URL.
type rebaseTransport struct {
	base string
	rt   http.RoundTripper
}

func (r rebaseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = r.base[len("http://"):]
	rt := r.rt
	if rt == nil {
		rt = http.DefaultTransport
	}
	return rt.RoundTrip(req2)
}

// ── normalizeDOI ──────────────────────────────────────────────────────────────

func TestNormalizeDOI(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"10.1000/xyz", "10.1000/xyz"},
		{"https://doi.org/10.1000/xyz", "10.1000/xyz"},
		{"http://doi.org/10.1000/xyz", "10.1000/xyz"},
		{"doi.org/10.1000/xyz", "10.1000/xyz"},
		{"2301.12345", ""}, // arXiv, not a DOI
		{"notadoi", ""},
		{"10.nodash", ""}, // no slash
	}
	for _, tc := range tests {
		if got := normalizeDOI(tc.in); got != tc.want {
			t.Errorf("normalizeDOI(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ── normalizeArxivID ──────────────────────────────────────────────────────────

func TestNormalizeArxivID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"2301.12345", "2301.12345"},
		{"2301.12345v2", "2301.12345v2"},
		{"hep-th/0603001", "hep-th/0603001"},
		{"https://arxiv.org/abs/2301.12345", "2301.12345"},
		{"http://arxiv.org/abs/hep-th/0603001", "hep-th/0603001"},
		{"10.1000/xyz", ""}, // DOI, not arXiv
		{"notanid", ""},
	}
	for _, tc := range tests {
		if got := normalizeArxivID(tc.in); got != tc.want {
			t.Errorf("normalizeArxivID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ── AddEntryFromID ────────────────────────────────────────────────────────────

func TestAddEntryFromID_DOI_CreatesEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeCrossrefJSON("Great Paper", "Jones", "Alice", "Nature", "2021", "10", "2", "1--5", "10.1/test"))
	}))
	defer srv.Close()
	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	dir := t.TempDir()
	key, _, err := AddEntryFromID("10.1/test", dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key == "" {
		t.Fatal("expected non-empty key")
	}
	c := loadCache(dir)
	entry, ok := c[key]
	if !ok {
		t.Fatalf("key %q not found in cache", key)
	}
	if entry.Source != "crossref" {
		t.Errorf("source = %q, want crossref", entry.Source)
	}
	if entry.Fields["doi"] != "10.1/test" {
		t.Errorf("doi = %q, want 10.1/test", entry.Fields["doi"])
	}
}

func TestAddEntryFromID_DOI_URLPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeCrossrefJSON("Paper", "Lee", "Bob", "Science", "2020", "5", "1", "10--20", "10.2/abc"))
	}))
	defer srv.Close()
	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	dir := t.TempDir()
	key, _, err := AddEntryFromID("https://doi.org/10.2/abc", dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := loadCache(dir)
	if _, ok := c[key]; !ok {
		t.Fatalf("key %q not in cache", key)
	}
}

func TestAddEntryFromID_ArxivID_CreatesEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write(makeArxivXMLWithCategory("Attention Is All You Need", "Vaswani, Ashish", "2017-06-12T00:00:00Z", "cs.LG"))
	}))
	defer srv.Close()
	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	dir := t.TempDir()
	key, _, err := AddEntryFromID("1706.03762", dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := loadCache(dir)
	entry, ok := c[key]
	if !ok {
		t.Fatalf("key %q not in cache", key)
	}
	if entry.Source != "arxiv" {
		t.Errorf("source = %q, want arxiv", entry.Source)
	}
	if entry.Fields["eprint"] != "1706.03762" {
		t.Errorf("eprint = %q, want 1706.03762", entry.Fields["eprint"])
	}
	if entry.Fields["primaryclass"] != "cs.LG" {
		t.Errorf("primaryclass = %q, want cs.LG", entry.Fields["primaryclass"])
	}
}

func TestAddEntryFromID_DOI_DeduplicatesExisting(t *testing.T) {
	// Seed cache with a DOI entry; second call must not hit the network.
	dir := t.TempDir()
	c := cache{"Jones2021Great": cacheEntry{
		Source: "crossref",
		Type:   "article",
		Fields: map[string]string{"doi": "10.1/dup"},
	}}
	saveCache(dir, c, nil)

	// No HTTP server — any network call would fail.
	key, _, err := AddEntryFromID("10.1/dup", dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "Jones2021Great" {
		t.Errorf("key = %q, want Jones2021Great", key)
	}
	// Cache must be unchanged.
	if got := len(loadCache(dir)); got != 1 {
		t.Errorf("cache size = %d, want 1", got)
	}
}

func TestAddEntryFromID_ArxivID_DeduplicatesExisting(t *testing.T) {
	dir := t.TempDir()
	c := cache{"Vaswani2017Attention": cacheEntry{
		Source: "arxiv",
		Type:   "misc",
		Fields: map[string]string{"eprint": "1706.03762"},
	}}
	saveCache(dir, c, nil)

	key, _, err := AddEntryFromID("1706.03762", dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "Vaswani2017Attention" {
		t.Errorf("key = %q, want Vaswani2017Attention", key)
	}
}

func TestAddEntryFromID_UnrecognizedID(t *testing.T) {
	dir := t.TempDir()
	_, _, err := AddEntryFromID("not-an-id", dir, nil)
	if err != ErrUnrecognizedID {
		t.Errorf("err = %v, want ErrUnrecognizedID", err)
	}
}

func TestValidateEntry_ArxivWithDOI_UsesCrossref(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api/query"):
			w.Header().Set("Content-Type", "application/xml")
			w.Write(makeArxivXMLWithDOI("Arxiv Title", "Smith, John", "2023-01-15T00:00:00Z", "10.1016/j.test.2023.1234"))
		case strings.Contains(r.URL.Path, "/works/"):
			w.Header().Set("Content-Type", "application/json")
			w.Write(makeCrossrefJSON("Crossref Title", "Smith", "John", "J. Testing", "2023", "1", "2", "10-20", "10.1016/j.test.2023.1234"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Type: "misc",
		Key:  "Smith2023",
		Fields: []Field{
			{Name: "title", Value: "{Arxiv Title}"},
			{Name: "eprint", Value: "{2301.00001}"},
			{Name: "archiveprefix", Value: "{arXiv}"},
		},
	}
	result, raw, source, warn := validateEntry(e, false, nil)
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	if source != "crossref" {
		t.Errorf("source = %q, want crossref", source)
	}
	if result == nil {
		t.Fatal("expected corrected entry, got nil")
	}
	if result.Type != "article" {
		t.Errorf("type = %q, want article", result.Type)
	}
	if got := FieldValue(*result, "title"); got != "Crossref Title" {
		t.Errorf("title = %q, want %q", got, "Crossref Title")
	}
	if got := raw.Fields["journal"]; got != "J. Testing" {
		t.Errorf("raw journal = %q, want %q", got, "J. Testing")
	}
	if got := raw.Fields["doi"]; got != "10.1016/j.test.2023.1234" {
		t.Errorf("raw doi = %q, want %q", got, "10.1016/j.test.2023.1234")
	}
}

func TestValidateEntry_ArxivWithDOI_CrossrefFails_FallsBackToArxiv(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api/query"):
			w.Header().Set("Content-Type", "application/xml")
			w.Write(makeArxivXMLWithDOI("Arxiv Title", "Smith, John", "2023-01-15T00:00:00Z", "10.1016/j.test.2023.1234"))
		case strings.Contains(r.URL.Path, "/works/"):
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	e := Entry{
		Type: "misc",
		Key:  "Smith2023",
		Fields: []Field{
			{Name: "title", Value: "{Wrong Title}"},
			{Name: "eprint", Value: "{2301.00001}"},
			{Name: "archiveprefix", Value: "{arXiv}"},
		},
	}
	result, _, source, warn := validateEntry(e, false, nil)
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	if source != "arxiv" {
		t.Errorf("source = %q, want arxiv (fallback)", source)
	}
	if result == nil {
		t.Fatal("expected corrected entry, got nil")
	}
	if got := FieldValue(*result, "title"); got != "Arxiv Title" {
		t.Errorf("title = %q, want %q (from arXiv fallback)", got, "Arxiv Title")
	}
}

func TestAddEntryFromID_ArxivWithDOI_UsesCrossref(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api/query"):
			w.Header().Set("Content-Type", "application/xml")
			w.Write(makeArxivXMLWithDOI("Arxiv Title", "Smith, John", "2023-01-15T00:00:00Z", "10.1016/j.test.2023.1234"))
		case strings.Contains(r.URL.Path, "/works/"):
			w.Header().Set("Content-Type", "application/json")
			w.Write(makeCrossrefJSON("Crossref Title", "Smith", "John", "J. Testing", "2023", "1", "2", "10-20", "10.1016/j.test.2023.1234"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	dir := t.TempDir()
	key, _, err := AddEntryFromID("2301.00001", dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := loadCache(dir)
	entry, ok := c[key]
	if !ok {
		t.Fatalf("key %q not in cache", key)
	}
	if entry.Source != "crossref" {
		t.Errorf("source = %q, want crossref", entry.Source)
	}
	if entry.Type != "article" {
		t.Errorf("type = %q, want article", entry.Type)
	}
	if got := entry.Fields["doi"]; got != "10.1016/j.test.2023.1234" {
		t.Errorf("doi = %q, want %q", got, "10.1016/j.test.2023.1234")
	}
	if got := entry.Fields["journal"]; got != "J. Testing" {
		t.Errorf("journal = %q, want %q", got, "J. Testing")
	}
}

func TestAddEntryFromID_ArxivWithDOI_DedupByDOI(t *testing.T) {
	// Seed cache with an entry that has the same DOI. The arXiv lookup should
	// find the DOI, then dedup against the existing cache entry.
	dir := t.TempDir()
	c := cache{"Smith2023Crossref": cacheEntry{
		Source: "crossref",
		Type:   "article",
		Fields: map[string]string{"doi": "10.1016/j.test.2023.1234"},
	}}
	saveCache(dir, c, nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api/query"):
			w.Header().Set("Content-Type", "application/xml")
			w.Write(makeArxivXMLWithDOI("A Title", "Smith, John", "2023-01-15T00:00:00Z", "10.1016/j.test.2023.1234"))
		default:
			// Crossref should NOT be called — dedup by DOI should hit cache.
			t.Error("unexpected request to", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	key, _, err := AddEntryFromID("2301.00001", dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "Smith2023Crossref" {
		t.Errorf("key = %q, want Smith2023Crossref", key)
	}
	// Cache must be unchanged.
	if got := len(loadCache(dir)); got != 1 {
		t.Errorf("cache size = %d, want 1", got)
	}
}

func TestAddEntryFromID_CorruptCache(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/bib.json", []byte("CORRUPT"), 0644)
	_, _, err := AddEntryFromID("10.1234/test", dir, nil)
	if err == nil {
		t.Fatal("expected error for corrupt cache, got nil")
	}
	if err != errCorruptCache {
		t.Errorf("expected errCorruptCache, got %v", err)
	}
	// Verify file was NOT overwritten.
	data, _ := os.ReadFile(dir + "/bib.json")
	if string(data) != "CORRUPT" {
		t.Error("corrupt bib.json was overwritten")
	}
}

func TestAddEntryFromID_DOI_APIFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	orig := httpClient
	httpClient = &http.Client{Transport: rebaseTransport{base: srv.URL}}
	defer func() { httpClient = orig }()

	dir := t.TempDir()
	_, _, err := AddEntryFromID("10.1/fail", dir, nil)
	if err == nil {
		t.Fatal("expected error for API failure, got nil")
	}
	if err == ErrUnrecognizedID {
		t.Error("API failure should not return ErrUnrecognizedID")
	}
}
