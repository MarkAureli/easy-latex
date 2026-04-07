package bib

import "testing"

// ── resolveAccent ─────────────────────────────────────────────────────────────

func TestResolveAccent_UmlautExpansion(t *testing.T) {
	cases := []struct{ cmd, letter, want string }{
		{`"`, "a", "ae"},
		{`"`, "A", "Ae"},
		{`"`, "o", "oe"},
		{`"`, "O", "Oe"},
		{`"`, "u", "ue"},
		{`"`, "U", "Ue"},
		{`"`, "e", "e"}, // no expansion for non-umlaut letters
	}
	for _, c := range cases {
		if got := resolveAccent(c.cmd, c.letter); got != c.want {
			t.Errorf("resolveAccent(%q, %q) = %q, want %q", c.cmd, c.letter, got, c.want)
		}
	}
}

func TestResolveAccent_OtherAccentsDropped(t *testing.T) {
	cases := []struct{ cmd, letter, want string }{
		{"'", "e", "e"},
		{"'", "E", "E"},
		{"`", "a", "a"},
		{"^", "o", "o"},
		{"~", "n", "n"},
		{"v", "c", "c"},
		{"c", "c", "c"}, // cedilla
	}
	for _, c := range cases {
		if got := resolveAccent(c.cmd, c.letter); got != c.want {
			t.Errorf("resolveAccent(%q, %q) = %q, want %q", c.cmd, c.letter, got, c.want)
		}
	}
}

// ── latexToASCII ──────────────────────────────────────────────────────────────

func TestLatexToASCII_MathModeStripped(t *testing.T) {
	cases := []struct{ in, want string }{
		{"$k$-Means", " -Means"},
		{"$$E = mc^2$$", " "},
		{"no math", "no math"},
	}
	for _, c := range cases {
		if got := latexToASCII(c.in); got != c.want {
			t.Errorf("latexToASCII(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLatexToASCII_UmlautLatex(t *testing.T) {
	cases := []struct{ in, want string }{
		{`M\"{u}ller`, "Mueller"},
		{`M\"uller`, "Mueller"},
		{`\"{O}ffentlich`, "Oeffentlich"},
		{`G\"{o}del`, "Goedel"},
	}
	for _, c := range cases {
		if got := latexToASCII(c.in); got != c.want {
			t.Errorf("latexToASCII(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLatexToASCII_AcuteLatex(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{\'e}tude`, "etude"},
		{`\'Etude`, "Etude"},
		{`\'{e}tude`, "etude"},
	}
	for _, c := range cases {
		if got := latexToASCII(c.in); got != c.want {
			t.Errorf("latexToASCII(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLatexToASCII_StandaloneCommands(t *testing.T) {
	cases := []struct{ in, want string }{
		{`\ss{}`, "ss"},
		{`\ae{}`, "ae"},
		{`\AE{}`, "Ae"},
		{`\oe{}`, "oe"},
		{`\OE{}`, "Oe"},
		{`\AA{}`, "A"},
	}
	for _, c := range cases {
		if got := latexToASCII(c.in); got != c.want {
			t.Errorf("latexToASCII(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLatexToASCII_CommandsWithArgStripped(t *testing.T) {
	if got := latexToASCII(`\textbf{Hello}`); got != "Hello" {
		t.Errorf("got %q, want %q", got, "Hello")
	}
}

func TestLatexToASCII_NestedCommandsStripped(t *testing.T) {
	if got := latexToASCII(`\textbf{\textit{Hello}}`); got != "Hello" {
		t.Errorf("got %q, want %q", got, "Hello")
	}
}

func TestLatexToASCII_UnicodeAccents(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Müller", "Mueller"},
		{"Ångström", "Angstroem"}, // Å→A, n, g, s, t, r, ö→oe, m
		{"naïve", "naive"},
		{"fiancée", "fiancee"},
		{"Ñoño", "Nono"},
		{"ß", "ss"},
	}
	for _, c := range cases {
		if got := latexToASCII(c.in); got != c.want {
			t.Errorf("latexToASCII(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── toCamelCase ───────────────────────────────────────────────────────────────

func TestToCamelCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Smith", "Smith"},
		{"van der Berg", "VanDerBerg"},
		{"Garcia-Lopez", "GarciaLopez"},
		{"A Great Paper on Things", "AGreatPaperOnThings"},
		{"  leading spaces  ", "LeadingSpaces"},
		{"", ""},
	}
	for _, c := range cases {
		if got := toCamelCase(c.in); got != c.want {
			t.Errorf("toCamelCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── firstAuthorLastName ───────────────────────────────────────────────────────

func TestFirstAuthorLastName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Smith, John", "Smith"},
		{"Van Der Berg, John", "Van Der Berg"},
		{"John Smith", "Smith"},
		{"Smith, John and Doe, Jane", "Smith"},
		{"John Smith and Jane Doe", "Smith"},
		{"García-López, Maria", "García-López"},
		{"", ""},
	}
	for _, c := range cases {
		if got := firstAuthorLastName(c.in); got != c.want {
			t.Errorf("firstAuthorLastName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── GenerateKey ───────────────────────────────────────────────────────────────

func TestGenerateKey_Basic(t *testing.T) {
	e := Entry{
		Key: "old",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "year", Value: "{2023}"},
			{Name: "title", Value: "{A Great Paper}"},
		},
	}
	if got := GenerateKey(e); got != "Smith2023AGreatPaper" {
		t.Errorf("got %q, want %q", got, "Smith2023AGreatPaper")
	}
}

func TestGenerateKey_UmlautInAuthor(t *testing.T) {
	e := Entry{
		Fields: []Field{
			{Name: "author", Value: `{M\"{u}ller, Hans}`},
			{Name: "year", Value: "{2020}"},
			{Name: "title", Value: "{Some Work}"},
		},
	}
	if got := GenerateKey(e); got != "Mueller2020SomeWork" {
		t.Errorf("got %q, want %q", got, "Mueller2020SomeWork")
	}
}

func TestGenerateKey_CompoundLastName(t *testing.T) {
	e := Entry{
		Fields: []Field{
			{Name: "author", Value: "{Van Der Berg, John}"},
			{Name: "year", Value: "{2021}"},
			{Name: "title", Value: "{Deep Learning}"},
		},
	}
	if got := GenerateKey(e); got != "VanDerBerg2021DeepLearning" {
		t.Errorf("got %q, want %q", got, "VanDerBerg2021DeepLearning")
	}
}

func TestGenerateKey_MathInTitle(t *testing.T) {
	e := Entry{
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "year", Value: "{2022}"},
			{Name: "title", Value: "{Optimal $k$-Means Clustering}"},
		},
	}
	got := GenerateKey(e)
	// $k$ is stripped; "-" is a word separator
	if got != "Smith2022OptimalMeansClustering" {
		t.Errorf("got %q, want %q", got, "Smith2022OptimalMeansClustering")
	}
}

func TestGenerateKey_FallsBackWhenFieldsMissing(t *testing.T) {
	e := Entry{Key: "original", Fields: []Field{{Name: "title", Value: "{T}"}}}
	if got := GenerateKey(e); got != "original" {
		t.Errorf("should fall back to original key, got %q", got)
	}
}

func TestGenerateKey_UnpublishedWithYear(t *testing.T) {
	e := Entry{
		Type: "unpublished",
		Key:  "old",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "year", Value: "{2023}"},
			{Name: "title", Value: "{A Draft}"},
			{Name: "note", Value: "{draft}"},
		},
	}
	if got := GenerateKey(e); got != "Smith2023ADraft" {
		t.Errorf("got %q, want %q", got, "Smith2023ADraft")
	}
}

func TestGenerateKey_UnpublishedWithoutYear(t *testing.T) {
	e := Entry{
		Type: "unpublished",
		Key:  "old",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "title", Value: "{A Draft}"},
			{Name: "note", Value: "{draft}"},
		},
	}
	if got := GenerateKey(e); got != "SmithADraft" {
		t.Errorf("got %q, want %q", got, "SmithADraft")
	}
}

func TestGenerateKey_NonUnpublishedFallsBackWithoutYear(t *testing.T) {
	e := Entry{
		Type: "book",
		Key:  "original",
		Fields: []Field{
			{Name: "author", Value: "{Smith, John}"},
			{Name: "title", Value: "{A Book}"},
		},
	}
	if got := GenerateKey(e); got != "original" {
		t.Errorf("should fall back to original key, got %q", got)
	}
}

// ── assignCanonicalKeys ───────────────────────────────────────────────────────

func makeArticleItem(key, author, year, title string) Item {
	return Item{IsEntry: true, Entry: Entry{
		Type: "article", Key: key,
		Fields: []Field{
			{Name: "author", Value: "{" + author + "}"},
			{Name: "year", Value: "{" + year + "}"},
			{Name: "title", Value: "{" + title + "}"},
		},
	}}
}

func TestAssignCanonicalKeys_UniqueKeys(t *testing.T) {
	items := []Item{
		makeArticleItem("old1", "Smith, John", "2023", "A Paper"),
		makeArticleItem("old2", "Doe, Jane", "2022", "Another Work"),
	}
	assignCanonicalKeys(items)
	if got := items[0].Entry.Key; got != "Smith2023APaper" {
		t.Errorf("items[0].Key = %q, want %q", got, "Smith2023APaper")
	}
	if got := items[1].Entry.Key; got != "Doe2022AnotherWork" {
		t.Errorf("items[1].Key = %q, want %q", got, "Doe2022AnotherWork")
	}
}

func TestAssignCanonicalKeys_DuplicatesGetSuffixes(t *testing.T) {
	items := []Item{
		makeArticleItem("x", "Smith, John", "2023", "Same Title"),
		makeArticleItem("y", "Smith, John", "2023", "Same Title"),
		makeArticleItem("z", "Smith, John", "2023", "Same Title"),
	}
	assignCanonicalKeys(items)
	keys := []string{items[0].Entry.Key, items[1].Entry.Key, items[2].Entry.Key}
	if keys[0] != "Smith2023SameTitlea" || keys[1] != "Smith2023SameTitleb" || keys[2] != "Smith2023SameTitlec" {
		t.Errorf("unexpected keys: %v", keys)
	}
}

func TestAssignCanonicalKeys_RawItemsSkipped(t *testing.T) {
	items := []Item{
		{IsEntry: false, Raw: "% comment\n"},
		makeArticleItem("old", "Smith, John", "2023", "A Paper"),
	}
	assignCanonicalKeys(items)
	if got := items[0].Raw; got != "% comment\n" {
		t.Errorf("raw item modified")
	}
	if got := items[1].Entry.Key; got != "Smith2023APaper" {
		t.Errorf("items[1].Key = %q, want %q", got, "Smith2023APaper")
	}
}
