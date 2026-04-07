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
	"conference": {
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
		fmt.Fprintf(&buf, "  %s%s = %s,\n", f.Name, pad, f.Value)
	}

	buf.WriteString("}\n")
	return buf.String()
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
