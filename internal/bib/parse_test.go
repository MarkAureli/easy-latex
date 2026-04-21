package bib

import (
	"testing"
)

// ── ParseFile ──────────────────────────────────────────────────────────────────

func TestParseFile_SingleEntry(t *testing.T) {
	input := `@article{Smith2023,
  author = {Smith, John},
  title = {A Title},
  year = {2023},
}`
	items := ParseFile(input)
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
	if !items[0].IsEntry {
		t.Errorf("expected entry, got raw item")
	}
	e := items[0].Entry
	if e.Type != "article" {
		t.Errorf("type = %q, want article", e.Type)
	}
	if e.Key != "Smith2023" {
		t.Errorf("key = %q, want Smith2023", e.Key)
	}
	if len(e.Fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(e.Fields))
	}
}

func TestParseFile_MultipleEntries(t *testing.T) {
	input := `@article{Key1, author = {A}}
@book{Key2, author = {B}}
@inproceedings{Key3, title = {T}}`
	items := ParseFile(input)
	// Extract just the entry items (parser includes whitespace as raw items)
	var entries []Entry
	for _, item := range items {
		if item.IsEntry {
			entries = append(entries, item.Entry)
		}
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Type != "article" {
		t.Errorf("entry 0 type = %q, want article", entries[0].Type)
	}
	if entries[1].Type != "book" {
		t.Errorf("entry 1 type = %q, want book", entries[1].Type)
	}
	if entries[2].Type != "inproceedings" {
		t.Errorf("entry 2 type = %q, want inproceedings", entries[2].Type)
	}
}

func TestParseFile_DifferentEntryTypes(t *testing.T) {
	cases := []struct {
		input       string
		expectedType string
	}{
		{`@article{k, author = {A}}`, "article"},
		{`@book{k, author = {A}}`, "book"},
		{`@inproceedings{k, author = {A}}`, "inproceedings"},
		{`@misc{k, author = {A}}`, "misc"},
		{`@ARTICLE{k, author = {A}}`, "article"}, // uppercase should be normalized
		{`@ArTiClE{k, author = {A}}`, "article"}, // mixed case
	}
	for _, c := range cases {
		items := ParseFile(c.input)
		if len(items) != 1 || !items[0].IsEntry {
			t.Errorf("failed to parse entry from %q", c.input)
			continue
		}
		if items[0].Entry.Type != c.expectedType {
			t.Errorf("type from %q = %q, want %q", c.input, items[0].Entry.Type, c.expectedType)
		}
	}
}

func TestParseFile_EntryWithWhitespaceAndComments(t *testing.T) {
	input := `% This is a comment
@article{Smith2023,
  author = {Smith, John},
}
% Another comment
`
	items := ParseFile(input)
	if len(items) != 3 {
		t.Errorf("expected 3 items (comment, entry, comment), got %d", len(items))
	}
	if items[0].IsEntry {
		t.Errorf("expected raw item for comment, got entry")
	}
	if !items[1].IsEntry {
		t.Errorf("expected entry at index 1")
	}
	if items[2].IsEntry {
		t.Errorf("expected raw item for trailing comment, got entry")
	}
}

// ── Field Values (braced, quoted, bare) ──────────────────────────────────────

func TestParseFile_BracedValue(t *testing.T) {
	input := `@article{k, title = {A Braced Title}}`
	items := ParseFile(input)
	e := items[0].Entry
	if len(e.Fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(e.Fields))
	}
	if e.Fields[0].Name != "title" {
		t.Errorf("field name = %q, want title", e.Fields[0].Name)
	}
	if e.Fields[0].Value != "{A Braced Title}" {
		t.Errorf("field value = %q, want {A Braced Title}", e.Fields[0].Value)
	}
}

func TestParseFile_QuotedValue(t *testing.T) {
	input := `@article{k, title = "A Quoted Title"}`
	items := ParseFile(input)
	e := items[0].Entry
	if e.Fields[0].Value != "{A Quoted Title}" {
		t.Errorf("quoted value normalized to %q, want {A Quoted Title}", e.Fields[0].Value)
	}
}

func TestParseFile_BareNumber(t *testing.T) {
	input := `@article{k, year = 2023}`
	items := ParseFile(input)
	e := items[0].Entry
	if e.Fields[0].Value != "{2023}" {
		t.Errorf("bare number normalized to %q, want {2023}", e.Fields[0].Value)
	}
}

func TestParseFile_NestedBraces(t *testing.T) {
	input := `@article{k, title = {A {Nested} Title}}`
	items := ParseFile(input)
	e := items[0].Entry
	if e.Fields[0].Value != "{A {Nested} Title}" {
		t.Errorf("nested braces = %q, want {A {Nested} Title}", e.Fields[0].Value)
	}
}

func TestParseFile_QuotedValueWithInnerBraces(t *testing.T) {
	input := `@article{k, title = "A {Title} with braces"}`
	items := ParseFile(input)
	e := items[0].Entry
	if e.Fields[0].Value != "{A {Title} with braces}" {
		t.Errorf("quoted with braces = %q, want {A {Title} with braces}", e.Fields[0].Value)
	}
}

// ── String Concatenation with # ────────────────────────────────────────────────

func TestParseFile_ConcatenationTwoBraced(t *testing.T) {
	input := `@article{k, month = {Jan} # {uary}}`
	items := ParseFile(input)
	e := items[0].Entry
	if len(e.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(e.Fields))
	}
	// concatRaw({Jan}, {uary}) -> {Jan} + uary + } -> {January}
	if e.Fields[0].Value != "{January}" {
		t.Errorf("concatenation = %q, want {January}", e.Fields[0].Value)
	}
}

func TestParseFile_ConcatenationBracedAndQuoted(t *testing.T) {
	input := `@article{k, month = {Jan} # "uary"}`
	items := ParseFile(input)
	e := items[0].Entry
	if e.Fields[0].Value != "{January}" {
		t.Errorf("braced # quoted = %q, want {January}", e.Fields[0].Value)
	}
}

func TestParseFile_ConcatenationThreeValues(t *testing.T) {
	input := `@article{k, month = {J} # {a} # {n}}`
	items := ParseFile(input)
	e := items[0].Entry
	if e.Fields[0].Value != "{Jan}" {
		t.Errorf("three-way concatenation = %q, want {Jan}", e.Fields[0].Value)
	}
}

// ── Entry Delimiters (braces vs parens) ────────────────────────────────────────

func TestParseFile_BracesDelimiter(t *testing.T) {
	input := `@article{key, author = {A}}`
	items := ParseFile(input)
	if len(items) != 1 || !items[0].IsEntry {
		t.Errorf("failed to parse entry with brace delimiters")
	}
}

func TestParseFile_ParensDelimiter(t *testing.T) {
	input := `@article(key, author = {A})`
	items := ParseFile(input)
	if len(items) != 1 || !items[0].IsEntry {
		t.Errorf("failed to parse entry with paren delimiters")
	}
	if items[0].Entry.Key != "key" {
		t.Errorf("key = %q, want key", items[0].Entry.Key)
	}
}

// ── Non-Entry Items (@comment, @string, @preamble) ──────────────────────────

func TestParseFile_Comment(t *testing.T) {
	input := `@comment{This is a comment}`
	items := ParseFile(input)
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
	if items[0].IsEntry {
		t.Errorf("@comment should be raw item, not entry")
	}
	if items[0].Raw == "" {
		t.Errorf("@comment raw content should not be empty")
	}
}

func TestParseFile_String(t *testing.T) {
	input := `@string{NATURE = "Nature"}`
	items := ParseFile(input)
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
	if items[0].IsEntry {
		t.Errorf("@string should be raw item, not entry")
	}
}

func TestParseFile_Preamble(t *testing.T) {
	input := `@preamble{"This will be output at the start of the bibliography."}`
	items := ParseFile(input)
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
	if items[0].IsEntry {
		t.Errorf("@preamble should be raw item, not entry")
	}
}

func TestParseFile_MixedEntryAndNonEntry(t *testing.T) {
	input := `@comment{comment}@article{k1, author = {A}}@string{X = "Y"}@book{k2, author = {B}}`
	items := ParseFile(input)
	// With no whitespace between items, we should have: comment, article, string, book
	var entryCount, rawCount int
	for _, item := range items {
		if item.IsEntry {
			entryCount++
		} else {
			rawCount++
		}
	}
	if entryCount != 2 {
		t.Errorf("expected 2 entries (@article and @book), got %d", entryCount)
	}
	if rawCount != 2 {
		t.Errorf("expected 2 raw items (@comment and @string), got %d", rawCount)
	}
}

// ── Edge Cases ──────────────────────────────────────────────────────────────

func TestParseFile_EmptyEntry(t *testing.T) {
	input := `@article{key}`
	items := ParseFile(input)
	if len(items) != 1 || !items[0].IsEntry {
		t.Errorf("expected to parse entry with no fields")
	}
	if len(items[0].Entry.Fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(items[0].Entry.Fields))
	}
}

func TestParseFile_EmptyEntryWithCommaAndSpace(t *testing.T) {
	input := `@article{key, }`
	items := ParseFile(input)
	if len(items) != 1 || !items[0].IsEntry {
		t.Errorf("expected to parse entry with trailing comma")
	}
	if len(items[0].Entry.Fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(items[0].Entry.Fields))
	}
}

func TestParseFile_MissingKey(t *testing.T) {
	input := `@article{, author = {A}}`
	items := ParseFile(input)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].IsEntry {
		// Empty key is technically parsed as an entry with empty key
		e := items[0].Entry
		if e.Key != "" {
			t.Errorf("key = %q, expected empty", e.Key)
		}
	}
}

func TestParseFile_TrailingCommaInFields(t *testing.T) {
	input := `@article{key,
  author = {A},
  title = {T},
}`
	items := ParseFile(input)
	e := items[0].Entry
	if len(e.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(e.Fields))
	}
}

func TestParseFile_FieldWithoutValue(t *testing.T) {
	input := `@article{key, author = {A}, title, year = {2023}}`
	items := ParseFile(input)
	e := items[0].Entry
	// title without value should be skipped, so we get author and year
	if len(e.Fields) != 2 {
		t.Errorf("expected 2 fields (malformed field skipped), got %d", len(e.Fields))
	}
	if e.Fields[0].Name != "author" {
		t.Errorf("field 0 name = %q, want author", e.Fields[0].Name)
	}
	if e.Fields[1].Name != "year" {
		t.Errorf("field 1 name = %q, want year", e.Fields[1].Name)
	}
}

func TestParseFile_UnclosedBrace(t *testing.T) {
	input := `@article{key, title = {Unclosed}`
	items := ParseFile(input)
	// Parser should recover and consume to end of string
	if len(items) == 0 {
		t.Errorf("expected to parse something from unclosed brace")
	}
}

func TestParseFile_EmptyInput(t *testing.T) {
	input := ""
	items := ParseFile(input)
	if len(items) != 0 {
		t.Errorf("expected 0 items from empty input, got %d", len(items))
	}
}

func TestParseFile_OnlyWhitespace(t *testing.T) {
	input := "   \n\t  \n"
	items := ParseFile(input)
	if len(items) != 1 {
		t.Errorf("expected 1 raw item from whitespace, got %d", len(items))
	}
	if items[0].IsEntry {
		t.Errorf("expected raw item, got entry")
	}
}

func TestParseFile_FieldNameCaseSensitivity(t *testing.T) {
	input := `@article{k, Author = {A}, TITLE = {T}, YeAr = {2023}}`
	items := ParseFile(input)
	e := items[0].Entry
	if e.Fields[0].Name != "author" {
		t.Errorf("field name not lowercased: %q", e.Fields[0].Name)
	}
	if e.Fields[1].Name != "title" {
		t.Errorf("field name not lowercased: %q", e.Fields[1].Name)
	}
	if e.Fields[2].Name != "year" {
		t.Errorf("field name not lowercased: %q", e.Fields[2].Name)
	}
}

func TestParseFile_FieldNameWithWhitespace(t *testing.T) {
	input := `@article{k,  author  = {A}}`
	items := ParseFile(input)
	e := items[0].Entry
	if e.Fields[0].Name != "author" {
		t.Errorf("field name = %q, want author", e.Fields[0].Name)
	}
}

// ── FieldValue ──────────────────────────────────────────────────────────────

func TestFieldValue_ExistingField(t *testing.T) {
	e := Entry{
		Type: "article",
		Key:  "k1",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "title", Value: "{A Title}"},
		},
	}
	if got := FieldValue(e, "author"); got != "Smith, John" {
		t.Errorf("FieldValue(author) = %q, want Smith, John", got)
	}
	if got := FieldValue(e, "title"); got != "A Title" {
		t.Errorf("FieldValue(title) = %q, want A Title", got)
	}
}

func TestFieldValue_MissingField(t *testing.T) {
	e := Entry{
		Type: "article",
		Key:  "k1",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
		},
	}
	if got := FieldValue(e, "title"); got != "" {
		t.Errorf("FieldValue(missing) = %q, want empty string", got)
	}
}

func TestFieldValue_BracedValue(t *testing.T) {
	e := Entry{
		Fields: []Field{
			{Name: "title", Value: "{A Braced Title}"},
		},
	}
	if got := FieldValue(e, "title"); got != "A Braced Title" {
		t.Errorf("got %q, want A Braced Title", got)
	}
}

func TestFieldValue_QuotedValue(t *testing.T) {
	e := Entry{
		Fields: []Field{
			{Name: "title", Value: "{A Quoted Title}"},
		},
	}
	if got := FieldValue(e, "title"); got != "A Quoted Title" {
		t.Errorf("got %q, want A Quoted Title", got)
	}
}

func TestFieldValue_MatchesCaseSensitively(t *testing.T) {
	e := Entry{
		Fields: []Field{
			{Name: "author", Value: "{A}"},
		},
	}
	// FieldValue does exact match (case-sensitive), so AUTHOR won't match author
	if got := FieldValue(e, "AUTHOR"); got != "" {
		t.Errorf("FieldValue(AUTHOR) = %q, want empty (exact match only)", got)
	}
	// But lowercase author should match
	if got := FieldValue(e, "author"); got != "A" {
		t.Errorf("FieldValue(author) = %q, want A", got)
	}
}

func TestFieldValue_EmptyValue(t *testing.T) {
	e := Entry{
		Fields: []Field{
			{Name: "title", Value: "{}"},
		},
	}
	if got := FieldValue(e, "title"); got != "" {
		t.Errorf("FieldValue on empty braces = %q, want empty string", got)
	}
}

// ── SetField ────────────────────────────────────────────────────────────────

func TestSetField_NewField(t *testing.T) {
	e := Entry{
		Type: "article",
		Key:  "k1",
		Fields: []Field{
			{Name: "author", Value: "{A}"},
		},
	}
	SetField(&e, "title", "{A Title}")
	if len(e.Fields) != 2 {
		t.Errorf("expected 2 fields after SetField, got %d", len(e.Fields))
	}
	if e.Fields[1].Name != "title" {
		t.Errorf("new field name = %q, want title", e.Fields[1].Name)
	}
	if e.Fields[1].Value != "{A Title}" {
		t.Errorf("new field value = %q, want {A Title}", e.Fields[1].Value)
	}
}

func TestSetField_UpdateExistingField(t *testing.T) {
	e := Entry{
		Type: "article",
		Key:  "k1",
		Fields: []Field{
			{Name: "author", Value: "{Old Author}"},
			{Name: "title", Value: "{Old Title}"},
		},
	}
	SetField(&e, "author", "{New Author}")
	if len(e.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(e.Fields))
	}
	if e.Fields[0].Value != "{New Author}" {
		t.Errorf("updated field value = %q, want {New Author}", e.Fields[0].Value)
	}
	if e.Fields[1].Name != "title" {
		t.Errorf("second field changed unexpectedly")
	}
}

func TestSetField_ExactMatch(t *testing.T) {
	e := Entry{
		Fields: []Field{
			{Name: "author", Value: "{A}"},
		},
	}
	// SetField does exact match (case-sensitive), so AUTHOR won't match author
	SetField(&e, "AUTHOR", "{B}")
	if len(e.Fields) != 2 {
		t.Errorf("expected 2 fields (AUTHOR added as new), got %d", len(e.Fields))
	}
	if e.Fields[0].Value != "{A}" {
		t.Errorf("expected author field to remain unchanged")
	}
	if e.Fields[1].Name != "AUTHOR" || e.Fields[1].Value != "{B}" {
		t.Errorf("expected AUTHOR field to be added")
	}
}

func TestSetField_OnEmptyEntry(t *testing.T) {
	e := Entry{Type: "article", Key: "k"}
	SetField(&e, "author", "{A}")
	if len(e.Fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(e.Fields))
	}
	if e.Fields[0].Name != "author" || e.Fields[0].Value != "{A}" {
		t.Errorf("field not set correctly")
	}
}

func TestSetField_MultipleUpdates(t *testing.T) {
	e := Entry{
		Type: "article",
		Key:  "k",
		Fields: []Field{
			{Name: "a", Value: "{1}"},
			{Name: "b", Value: "{2}"},
			{Name: "c", Value: "{3}"},
		},
	}
	SetField(&e, "b", "{updated}")
	if e.Fields[1].Value != "{updated}" {
		t.Errorf("expected field b to be updated")
	}
	if len(e.Fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(e.Fields))
	}
}

// ── Integration and Roundtrip Tests ─────────────────────────────────────────

func TestParseFile_RoundtripSimple(t *testing.T) {
	input := `@article{Smith2023, author = {Smith, John}, title = {A Paper}, year = {2023}}`
	items := ParseFile(input)
	var entry Entry
	for _, item := range items {
		if item.IsEntry {
			entry = item.Entry
			break
		}
	}
	if entry.Type != "article" || entry.Key != "Smith2023" || len(entry.Fields) != 3 {
		t.Fatalf("entry not parsed correctly: type=%q, key=%q, fields=%d", entry.Type, entry.Key, len(entry.Fields))
	}
	// Now render and parse again to ensure consistency
	rendered := RenderEntries([]Entry{entry})
	items2 := ParseFile(rendered)
	var found bool
	for _, item := range items2 {
		if item.IsEntry {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("failed to parse rendered entry")
	}
}

func TestParseFile_ComplexDocument(t *testing.T) {
	input := `% header comment
@comment{Another comment}
@article{Key1, author = {Author One}, title = {Title One}, year = 2020}
@book{Key2, author = {Author Two}, title = {Title Two}}
@string{NATURE = "Nature"}
@inproceedings{Key3, author = {Author Three}, title = {Title Three}}`
	items := ParseFile(input)
	var entries int
	var rawItems int
	for _, item := range items {
		if item.IsEntry {
			entries++
		} else {
			rawItems++
		}
	}
	if entries != 3 {
		t.Errorf("expected 3 entries, got %d", entries)
	}
	// Raw items include: header comment, @comment, whitespace, @string, whitespace
	if rawItems < 2 {
		t.Errorf("expected at least 2 raw items, got %d", rawItems)
	}
}
