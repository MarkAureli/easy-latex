package bib

import (
	"bytes"
	"fmt"
	"strings"
)

// canonicalOrder defines the preferred field ordering per entry type.
// Fields not in the list are appended at the end in their original order.
var canonicalOrder = map[string][]string{
	"article": {
		"author", "year", "title", "journal",
		"volume", "number", "pages", "doi", "url",
	},
	"book": {
		"author", "year", "title", "publisher", "address", "doi", "url",
	},
	"incollection": {
		"author", "year", "title", "booktitle", "publisher", "address", "pages", "doi", "url",
	},
	"inproceedings": {
		"author", "year", "title", "booktitle", "pages", "doi", "url",
	},
	"phdthesis": {
		"author", "year", "title", "school", "doi", "url",
	},
	"mastersthesis": {
		"author", "year", "title", "school", "doi", "url",
	},
	"techreport": {
		"author", "year", "title", "institution", "doi", "url",
	},
	"misc": {
		"author", "year", "title", "eprint", "archiveprefix", "primaryclass", "doi", "url",
	},
	"unpublished": {
		"author", "year", "title", "doi", "url", "note",
	},
}

func init() {
	canonicalOrder["conference"] = canonicalOrder["inproceedings"]
}

// RenderEntries formats a slice of entries into .bib file content,
// separated by blank lines.
func RenderEntries(entries []Entry) string {
	var buf bytes.Buffer
	for i, e := range entries {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(formatEntry(e))
	}
	return buf.String()
}

// renderItems converts a slice of Items back to .bib file content.
// Whitespace-only raw items are collapsed to a single newline.
// A blank line is always emitted between consecutive entries.
func renderItems(items []Item) string {
	var buf bytes.Buffer
	for i, item := range items {
		if item.IsEntry {
			if i > 0 && items[i-1].IsEntry {
				buf.WriteByte('\n')
			}
			buf.WriteString(formatEntry(item.Entry))
		} else {
			if strings.TrimSpace(item.Raw) == "" {
				buf.WriteByte('\n')
			} else {
				buf.WriteString(item.Raw)
			}
		}
	}
	return buf.String()
}

func formatEntry(e Entry) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "@%s{%s,\n", e.Type, e.Key)

	fields := sortedFields(e.Type, e.Fields)

	maxLen := 0
	for _, f := range fields {
		if len(f.Name) > maxLen {
			maxLen = len(f.Name)
		}
	}

	for _, f := range fields {
		pad := strings.Repeat(" ", maxLen-len(f.Name))
		fmt.Fprintf(&buf, "  %s%s = %s,\n", f.Name, pad, escapePercent(escapeUnderscore(escapeAmpersand(escapeUnicode(f.Value)))))
	}

	buf.WriteString("}\n")
	return buf.String()
}

// stripNonEscapedBraces removes all { and } characters from s that are not
// immediately preceded by a backslash. Used to normalise title field values.
func stripNonEscapedBraces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if (s[i] == '{' || s[i] == '}') && (i == 0 || s[i-1] != '\\') {
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// unicodeToLaTeX maps Unicode accented characters to their LaTeX equivalents.
// This is conceptually the inverse of the latexToASCII path in key.go, but
// produces proper LaTeX commands rather than stripped ASCII.
var unicodeToLaTeX = map[rune]string{
	// Acute \'
	'á': `{\'a}`, 'Á': `{\'A}`,
	'é': `{\'e}`, 'É': `{\'E}`,
	'í': `{\'i}`, 'Í': `{\'I}`,
	'ó': `{\'o}`, 'Ó': `{\'O}`,
	'ú': `{\'u}`, 'Ú': `{\'U}`,
	'ý': `{\'y}`, 'Ý': `{\'Y}`,
	'ć': `{\'c}`, 'Ć': `{\'C}`,
	'ś': `{\'s}`, 'Ś': `{\'S}`,
	'ź': `{\'z}`, 'Ź': `{\'Z}`,
	'ĺ': `{\'l}`, 'Ĺ': `{\'L}`,

	// Grave \`
	'à': "{\\`a}", 'À': "{\\`A}",
	'è': "{\\`e}", 'È': "{\\`E}",
	'ì': "{\\`i}", 'Ì': "{\\`I}",
	'ò': "{\\`o}", 'Ò': "{\\`O}",
	'ù': "{\\`u}", 'Ù': "{\\`U}",

	// Circumflex \^
	'â': `{\^a}`, 'Â': `{\^A}`,
	'ê': `{\^e}`, 'Ê': `{\^E}`,
	'î': `{\^i}`, 'Î': `{\^I}`,
	'ô': `{\^o}`, 'Ô': `{\^O}`,
	'û': `{\^u}`, 'Û': `{\^U}`,

	// Tilde \~
	'ã': `{\~a}`, 'Ã': `{\~A}`,
	'õ': `{\~o}`, 'Õ': `{\~O}`,
	'ñ': `{\~n}`, 'Ñ': `{\~N}`,

	// Umlaut \"
	'ä': `{\"a}`, 'Ä': `{\"A}`,
	'ë': `{\"e}`, 'Ë': `{\"E}`,
	'ï': `{\"i}`, 'Ï': `{\"I}`,
	'ö': `{\"o}`, 'Ö': `{\"O}`,
	'ü': `{\"u}`, 'Ü': `{\"U}`,
	'ÿ': `{\"y}`,

	// Macron \=
	'ā': `{\=a}`, 'Ā': `{\=A}`,
	'ē': `{\=e}`, 'Ē': `{\=E}`,
	'ī': `{\=i}`, 'Ī': `{\=I}`,
	'ō': `{\=o}`, 'Ō': `{\=O}`,
	'ū': `{\=u}`, 'Ū': `{\=U}`,

	// Dot above \.
	'ż': `{\.z}`, 'Ż': `{\.Z}`,

	// Caron \v{}
	'č': `{\v{c}}`, 'Č': `{\v{C}}`,
	'ě': `{\v{e}}`, 'Ě': `{\v{E}}`,
	'š': `{\v{s}}`, 'Š': `{\v{S}}`,
	'ž': `{\v{z}}`, 'Ž': `{\v{Z}}`,
	'ř': `{\v{r}}`, 'Ř': `{\v{R}}`,
	'ď': `{\v{d}}`, 'Ď': `{\v{D}}`,
	'ť': `{\v{t}}`, 'Ť': `{\v{T}}`,
	'ň': `{\v{n}}`, 'Ň': `{\v{N}}`,
	'ľ': `{\v{l}}`, 'Ľ': `{\v{L}}`,

	// Breve \u{}
	'ă': `{\u{a}}`, 'Ă': `{\u{A}}`,
	'ĕ': `{\u{e}}`, 'Ĕ': `{\u{E}}`,
	'ĭ': `{\u{i}}`, 'Ĭ': `{\u{I}}`,
	'ŏ': `{\u{o}}`, 'Ŏ': `{\u{O}}`,
	'ŭ': `{\u{u}}`, 'Ŭ': `{\u{U}}`,

	// Ring above \r{}
	'ů': `{\r{u}}`, 'Ů': `{\r{U}}`,

	// Double acute \H{}
	'ő': `{\H{o}}`, 'Ő': `{\H{O}}`,
	'ű': `{\H{u}}`, 'Ű': `{\H{U}}`,

	// Cedilla \c{}
	'ç': `{\c{c}}`, 'Ç': `{\c{C}}`,
	'ļ': `{\c{l}}`, 'Ļ': `{\c{L}}`,

	// Ogonek \k{}
	'ą': `{\k{a}}`, 'Ą': `{\k{A}}`,
	'ę': `{\k{e}}`, 'Ę': `{\k{E}}`,
	'į': `{\k{i}}`, 'Į': `{\k{I}}`,
	'ų': `{\k{u}}`, 'Ų': `{\k{U}}`,

	// Standalone commands
	'ß': `{\ss}`,
	'æ': `{\ae}`, 'Æ': `{\AE}`,
	'œ': `{\oe}`, 'Œ': `{\OE}`,
	'å': `{\aa}`, 'Å': `{\AA}`,
	'ø': `{\o}`, 'Ø': `{\O}`,
	'ł': `{\l}`, 'Ł': `{\L}`,
	'ð': `{\dh}`, 'Ð': `{\DH}`,
	'þ': `{\th}`, 'Þ': `{\TH}`,
	'ŋ': `{\ng}`,
}

// escapeUnicode replaces Unicode accented characters in a bib field value
// with their LaTeX escape sequences (e.g. ü → {\"u}, Č → {\v{C}}).
func escapeUnicode(s string) string {
	needsEscape := false
	for _, r := range s {
		if _, ok := unicodeToLaTeX[r]; ok {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	for _, r := range s {
		if repl, ok := unicodeToLaTeX[r]; ok {
			b.WriteString(repl)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// escapeAmpersand replaces unescaped & with \& inside a bib field value so
// that bibtex/biber does not emit a bare & into the .bbl, which LaTeX would
// interpret as a table column separator and fail.
func escapeAmpersand(v string) string {
	if !strings.Contains(v, "&") {
		return v
	}
	var b strings.Builder
	b.Grow(len(v) + 4)
	for i := 0; i < len(v); i++ {
		if v[i] == '&' && (i == 0 || v[i-1] != '\\') {
			b.WriteString(`\&`)
		} else {
			b.WriteByte(v[i])
		}
	}
	return b.String()
}

// escapePercent replaces unescaped % with \% in a bib field value so that
// BibTeX does not treat the rest of the line as a comment.
func escapePercent(v string) string {
	if !strings.Contains(v, "%") {
		return v
	}
	var b strings.Builder
	b.Grow(len(v) + 4)
	for i := 0; i < len(v); i++ {
		if v[i] == '%' && (i == 0 || v[i-1] != '\\') {
			b.WriteString(`\%`)
		} else {
			b.WriteByte(v[i])
		}
	}
	return b.String()
}

// escapeUnderscore replaces unescaped _ with \_ in a bib field value so that
// LaTeX does not interpret them as subscript operators in text mode.
func escapeUnderscore(v string) string {
	if !strings.Contains(v, "_") {
		return v
	}
	var b strings.Builder
	b.Grow(len(v) + 4)
	for i := 0; i < len(v); i++ {
		if v[i] == '_' && (i == 0 || v[i-1] != '\\') {
			b.WriteString(`\_`)
		} else {
			b.WriteByte(v[i])
		}
	}
	return b.String()
}

// sortedFields returns fields in canonical order for the given entry type,
// with any unrecognized fields appended in their original order.
func sortedFields(entryType string, fields []Field) []Field {
	order, ok := canonicalOrder[entryType]
	if !ok {
		return fields
	}

	fieldByName := make(map[string]Field, len(fields))
	for _, f := range fields {
		fieldByName[f.Name] = f
	}

	result := make([]Field, 0, len(fields))
	for _, name := range order {
		if f, found := fieldByName[name]; found {
			result = append(result, f)
			delete(fieldByName, name)
		}
	}
	for _, f := range fields {
		if _, remaining := fieldByName[f.Name]; remaining {
			result = append(result, f)
		}
	}
	return result
}
