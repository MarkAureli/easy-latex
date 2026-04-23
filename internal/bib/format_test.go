package bib

import (
	"strings"
	"testing"
)

// ── sortedFields ──────────────────────────────────────────────────────────────

func TestSortedFields_KnownType(t *testing.T) {
	fields := []Field{
		{Name: "doi", Value: "{10.1000/xyz}"},
		{Name: "year", Value: "{2020}"},
		{Name: "title", Value: "{A Title}"},
		{Name: "author", Value: "{Smith, John}"},
		{Name: "journal", Value: "{Nature}"},
	}
	got := sortedFields("article", fields)
	want := []string{"author", "year", "title", "journal", "doi"}
	for i, f := range got {
		if f.Name != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, f.Name, want[i])
		}
	}
}

func TestSortedFields_UnknownTypePreservesOrder(t *testing.T) {
	fields := []Field{
		{Name: "z", Value: "{z}"},
		{Name: "a", Value: "{a}"},
		{Name: "m", Value: "{m}"},
	}
	got := sortedFields("unknown", fields)
	for i, f := range got {
		if f.Name != fields[i].Name {
			t.Errorf("got[%d] = %q, want %q", i, f.Name, fields[i].Name)
		}
	}
}

func TestSortedFields_ExtraFieldsAppendedInOrder(t *testing.T) {
	fields := []Field{
		{Name: "custom1", Value: "{v1}"},
		{Name: "title", Value: "{T}"},
		{Name: "custom2", Value: "{v2}"},
		{Name: "author", Value: "{A}"},
	}
	got := sortedFields("article", fields)
	// author and title must come first (canonical order for article)
	if got[0].Name != "author" || got[1].Name != "title" {
		t.Errorf("canonical fields not at the front: %v", got)
	}
	// custom fields must appear after canonical ones, in original relative order
	extras := got[len(got)-2:]
	if extras[0].Name != "custom1" || extras[1].Name != "custom2" {
		t.Errorf("extra fields wrong order: %v", extras)
	}
}

// ── formatEntry ───────────────────────────────────────────────────────────────

func TestFormatEntry_Basic(t *testing.T) {
	e := Entry{
		Type: "article",
		Key:  "Smith2023",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "title", Value: "{A Great Paper}"},
			{Name: "journal", Value: "{Nature}"},
			{Name: "year", Value: "{2023}"},
		},
	}
	out := formatEntry(e)

	if !strings.HasPrefix(out, "@article{Smith2023,\n") {
		t.Errorf("unexpected header: %q", out[:min(50, len(out))])
	}
	if !strings.HasSuffix(out, "}\n") {
		t.Errorf("entry does not end with }\\n")
	}
	// All fields end with a comma (including last)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n")[1 : len(strings.Split(strings.TrimSpace(out), "\n"))-1] {
		if !strings.HasSuffix(line, ",") {
			t.Errorf("field line missing trailing comma: %q", line)
		}
	}
}

func TestFormatEntry_FieldAlignment(t *testing.T) {
	e := Entry{
		Type: "article",
		Key:  "X",
		Fields: []Field{
			{Name: "author", Value: "{A}"},
			{Name: "doi", Value: "{10.1}"},
		},
	}
	out := formatEntry(e)
	lines := strings.Split(out, "\n")
	// Find the two field lines and check that '=' signs are aligned
	var fieldLines []string
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "author") || strings.HasPrefix(strings.TrimSpace(l), "doi") {
			fieldLines = append(fieldLines, l)
		}
	}
	if len(fieldLines) != 2 {
		t.Fatalf("expected 2 field lines, got %d", len(fieldLines))
	}
	eqPos := func(s string) int { return strings.Index(s, "=") }
	if eqPos(fieldLines[0]) != eqPos(fieldLines[1]) {
		t.Errorf("'=' not aligned: %q vs %q", fieldLines[0], fieldLines[1])
	}
}

// ── renderItems ───────────────────────────────────────────────────────────────

func TestRenderItems_BlankLineBetweenEntries(t *testing.T) {
	items := []Item{
		{IsEntry: true, Entry: Entry{Type: "article", Key: "A", Fields: []Field{{Name: "author", Value: "{A}"}}}},
		{IsEntry: true, Entry: Entry{Type: "article", Key: "B", Fields: []Field{{Name: "author", Value: "{B}"}}}},
	}
	out := renderItems(items)
	// There should be a blank line between the two entries
	if !strings.Contains(out, "}\n\n@") {
		t.Errorf("no blank line between entries:\n%s", out)
	}
}

func TestRenderItems_WhitespaceOnlyRawCollapsed(t *testing.T) {
	items := []Item{
		{IsEntry: false, Raw: "   \n\t  \n"},
		{IsEntry: true, Entry: Entry{Type: "misc", Key: "k", Fields: nil}},
	}
	out := renderItems(items)
	// Whitespace-only raw item must become exactly one newline
	if !strings.HasPrefix(out, "\n") {
		t.Errorf("expected leading newline, got %q", out[:min(20, len(out))])
	}
}

func TestRenderItems_NonWhitespaceRawPreserved(t *testing.T) {
	comment := "% a comment\n"
	items := []Item{
		{IsEntry: false, Raw: comment},
		{IsEntry: true, Entry: Entry{Type: "misc", Key: "k"}},
	}
	out := renderItems(items)
	if !strings.HasPrefix(out, comment) {
		t.Errorf("comment not preserved at start")
	}
}

// ── escapeUnicode ────────────────────────────────────────────────────────────

func TestEscapeUnicode(t *testing.T) {
	cases := []struct{ in, want string }{
		// No-op for plain ASCII
		{"{Smith, John}", "{Smith, John}"},
		// Caron
		{"{Čech}", `{{\v{C}}ech}`},
		// Umlaut
		{"{Müller}", `{M{\"u}ller}`},
		// Acute
		{"{García}", `{Garc{\'i}a}`},
		// Grave
		{"{càfe}", "{c{\\`a}fe}"},
		// Tilde
		{"{Muñoz}", `{Mu{\~n}oz}`},
		// Circumflex
		{"{Hôtel}", `{H{\^o}tel}`},
		// Standalone: ß, ø, æ, ł
		{"{Straße}", `{Stra{\ss}e}`},
		{"{Sørensen}", `{S{\o}rensen}`},
		{"{Ælfred}", `{{\AE}lfred}`},
		{"{Łódź}", `{{\L}{\'o}d{\'z}}`},
		// Multiple in one string
		{"{Ärgerlich über Öl}", `{{\"A}rgerlich {\"u}ber {\"O}l}`},
		// Already-ASCII LaTeX commands left alone
		{`{Sm{\v{c}}ith}`, `{Sm{\v{c}}ith}`},
	}
	for _, c := range cases {
		if got := escapeUnicode(c.in); got != c.want {
			t.Errorf("escapeUnicode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatEntry_EscapesUnicode(t *testing.T) {
	e := Entry{
		Type: "article",
		Key:  "Cech2023",
		Fields: []Field{
			{Name: "author", Value: "{Čech, Tomáš}"},
			{Name: "title", Value: "{A Title}"},
			{Name: "year", Value: "{2023}"},
		},
	}
	out := formatEntry(e)
	if !strings.Contains(out, `{\v{C}}ech`) {
		t.Errorf("Č not escaped in output:\n%s", out)
	}
	if !strings.Contains(out, `Tom{\'a}{\v{s}}`) {
		t.Errorf("áš not escaped in output:\n%s", out)
	}
}

// ── stripNonEscapedBraces ─────────────────────────────────────────────────────

func TestStripNonEscapedBraces(t *testing.T) {
	cases := []struct{ in, want string }{
		{"No braces here", "No braces here"},
		{"Some {Special} Title", "Some Special Title"},
		{"{Leading} brace", "Leading brace"},
		{"Trailing {brace}", "Trailing brace"},
		{"Escaped \\{brace\\}", "Escaped \\{brace\\}"},
		{"Mixed {inner} and \\{escaped\\}", "Mixed inner and \\{escaped\\}"},
		{"", ""},
		{"{}", ""},
	}
	for _, c := range cases {
		if got := stripNonEscapedBraces(c.in); got != c.want {
			t.Errorf("stripNonEscapedBraces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── roundtrip formatting ──────────────────────────────────────────────────────

func TestFormatIdempotent(t *testing.T) {
	input := `@article{Smith2023,
  author  = {Smith, John},
  year    = {2023},
  title   = {A Great Paper},
  journal = {Nature},
}
`
	items := ParseFile(input)
	first := renderItems(items)
	items2 := ParseFile(first)
	second := renderItems(items2)
	if first != second {
		t.Errorf("formatting not idempotent.\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

