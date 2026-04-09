package bib

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
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
	result, err := queryCrossref(e, "10.1000/xyz", true)
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
	result, err := queryCrossref(e, "10.1/x", true)
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
	normalizeEntryFields(&e)
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
	normalizeEntryFields(&e)
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
	normalizeEntryFields(&e)
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
	normalizeEntryFields(&e)
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
	normalizeEntryFields(&e)
	if got := FieldValue(e, "url"); got != "https://example.com" {
		t.Errorf("url overwritten: got %q", got)
	}
}

func TestNormalizeEntryFields_UnknownTypeUnchanged(t *testing.T) {
	e := Entry{
		Type:   "unknown",
		Fields: []Field{{Name: "note", Value: "{kept}"}},
	}
	normalizeEntryFields(&e)
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
	normalizeEntryFields(&e)
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
			{Name: "archiveprefix", Value: "{arXiv}"},
		},
	}
	normalizeEntryFields(&e)
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
	normalizeEntryFields(&e)
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
	corrected, source, warn := validateEntry(e, true)
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
	_, source, warn := validateEntry(e, true)
	if source != "no-id" {
		t.Errorf("source = %q, want %q", source, "no-id")
	}
	if warn != "" {
		t.Errorf("expected no warning for book without doi, got %q", warn)
	}
}

// ── brace titles ─────────────────────────────────────────────────────────────

func TestBraceTitles_AppliesDoublebraces(t *testing.T) {
	dir := t.TempDir()
	bib := `@article{Smith2023Test,
  author  = {Smith, John},
  year    = {2023},
  title   = {My Test Title},
  journal = {Nature},
  doi     = {10.1/x},
  url     = {https://doi.org/10.1/x},
}
`
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bib), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ProcessBibFiles([]string{path}, dir, true, true, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
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
	// Start with an already double-braced title (as written by a previous run).
	bib := `@article{Smith2023Test,
  author  = {Smith, John},
  year    = {2023},
  title   = {{My Test Title}},
  journal = {Nature},
  doi     = {10.1/x},
  url     = {https://doi.org/10.1/x},
}
`
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bib), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ProcessBibFiles([]string{path}, dir, true, true, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
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
		t.Errorf("title raw value = %q, want %q (not idempotent)", raw, "{{My Test Title}}")
	}
}

func TestBraceTitles_Disabled_LeavesTitle(t *testing.T) {
	dir := t.TempDir()
	bib := `@article{Smith2023Test,
  author  = {Smith, John},
  year    = {2023},
  title   = {My Test Title},
  journal = {Nature},
  doi     = {10.1/x},
  url     = {https://doi.org/10.1/x},
}
`
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bib), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ProcessBibFiles([]string{path}, dir, true, false, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
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

func TestBraceTitles_DisabledNormalizesDoubleBraced(t *testing.T) {
	dir := t.TempDir()
	// If braceTitles is later disabled, existing double-braced titles should be stripped.
	bib := `@article{Smith2023Test,
  author  = {Smith, John},
  year    = {2023},
  title   = {{My Test Title}},
  journal = {Nature},
  doi     = {10.1/x},
  url     = {https://doi.org/10.1/x},
}
`
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bib), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ProcessBibFiles([]string{path}, dir, true, false, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
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
		t.Errorf("title raw value = %q, want %q (double braces not stripped)", raw, "{My Test Title}")
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

func TestIEEEFormat_ArxivMiscBecomesUnpublished(t *testing.T) {
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

	if err := ProcessBibFiles([]string{path}, dir, true, false, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	items := ParseFile(string(data))
	var entry Entry
	for _, it := range items {
		if it.IsEntry {
			entry = it.Entry
			break
		}
	}

	if entry.Type != "unpublished" {
		t.Errorf("type = %q, want %q", entry.Type, "unpublished")
	}
	for _, name := range []string{"eprint", "archiveprefix", "primaryclass"} {
		if FieldValue(entry, name) != "" {
			t.Errorf("field %q should have been dropped", name)
		}
	}
	note := FieldValue(entry, "note")
	if !strings.Contains(note, "2301.00001") {
		t.Errorf("note %q does not reference eprint", note)
	}
	if !strings.Contains(note, `\href`) {
		t.Errorf("note %q does not contain \\href", note)
	}
}

func TestIEEEFormat_ForcesBraceTitles(t *testing.T) {
	dir := t.TempDir()
	bibContent := `@article{Smith2023Test,
  author  = {Smith, John},
  year    = {2023},
  title   = {My Test Title},
  journal = {Nature},
  doi     = {10.1/x},
  url     = {https://doi.org/10.1/x},
}
`
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bibContent), 0644); err != nil {
		t.Fatal(err)
	}

	// ieee_format=true, brace_titles=false — should still double-brace
	if err := ProcessBibFiles([]string{path}, dir, true, false, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
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

func TestIEEEFormat_NonArxivMiscUnchanged(t *testing.T) {
	dir := t.TempDir()
	bibContent := `@misc{Smith2023Software,
  author = {Smith, John},
  year   = {2023},
  title  = {Some Software},
  url    = {https://example.com},
}
`
	path := dir + "/test.bib"
	if err := os.WriteFile(path, []byte(bibContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ProcessBibFiles([]string{path}, dir, true, false, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
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
