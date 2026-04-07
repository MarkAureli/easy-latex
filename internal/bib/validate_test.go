package bib

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
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
	}
	type response struct {
		Status  string  `json:"status"`
		Message message `json:"message"`
	}
	r := response{
		Status: "ok",
		Message: message{
			Title:          []string{title},
			Author:         []author{{Family: family, Given: given}},
			ContainerTitle: []string{container},
			Published:      published{DateParts: [][]int{{2023}}},
			Volume:         volume,
			Issue:          issue,
			Page:           page,
			DOI:            doi,
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
	result, err := queryCrossref(e, "10.1000/xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected correction, got nil")
	}
	if got := FieldValue(*result, "title"); got != "Correct Title" {
		t.Errorf("title = %q, want %q", got, "Correct Title")
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
	result, err := queryCrossref(e, "10.1/x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil (no change), got corrected entry")
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

func splitAuthors(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
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
	result, err := queryArxiv(e, "2301.00001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected correction, got nil")
	}
	if got := FieldValue(*result, "title"); got != "Correct Title" {
		t.Errorf("title = %q, want %q", got, "Correct Title")
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
	result, err := queryArxiv(e, "1906.00001")
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

// ── validateEntry ─────────────────────────────────────────────────────────────

func TestValidateEntry_NoIDWarning(t *testing.T) {
	e := Entry{Key: "NoID", Fields: []Field{{Name: "title", Value: "{X}"}}}
	corrected, source, warn := validateEntry(e)
	if corrected != nil {
		t.Error("expected no correction")
	}
	if source != "no-id" {
		t.Errorf("source = %q, want %q", source, "no-id")
	}
	if warn == "" {
		t.Error("expected a warning for no-id entry")
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
