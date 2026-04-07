package bib

import "strings"

// Entry represents a single BibTeX entry.
type Entry struct {
	Type   string  // lowercase entry type, e.g. "article"
	Key    string  // citation key
	Fields []Field // ordered fields
}

// Field is a single name=value pair within an entry.
type Field struct {
	Name  string // lowercase field name
	Value string // value normalized to {braced} form
}

// Item is either a parsed entry or a raw chunk of text (whitespace, comments,
// @string, @preamble declarations).
type Item struct {
	IsEntry bool
	Entry   Entry
	Raw     string // non-empty when IsEntry == false
}

// ParseFile parses the full content of a .bib file into a sequence of Items.
func ParseFile(content string) []Item {
	var items []Item
	pos := 0
	textStart := 0

	for {
		i := strings.IndexByte(content[pos:], '@')
		if i < 0 {
			break
		}
		i += pos

		if i > textStart {
			items = append(items, Item{Raw: content[textStart:i]})
		}

		end, entry, ok := parseAtBlock(content, i)
		if ok {
			items = append(items, Item{IsEntry: true, Entry: entry})
		} else {
			items = append(items, Item{Raw: content[i:end]})
		}
		textStart = end
		pos = end
		if pos >= len(content) {
			break
		}
	}

	if textStart < len(content) {
		items = append(items, Item{Raw: content[textStart:]})
	}
	return items
}

// parseAtBlock parses one @-block starting at pos (content[pos] == '@').
// Returns (end, entry, isEntry).
func parseAtBlock(content string, pos int) (int, Entry, bool) {
	pos++ // skip '@'

	typeStart := pos
	for pos < len(content) && isIdent(content[pos]) {
		pos++
	}
	entryType := strings.ToLower(content[typeStart:pos])

	pos = skipWS(content, pos)
	if pos >= len(content) {
		return pos, Entry{}, false
	}

	var closeDelim byte
	switch content[pos] {
	case '{':
		closeDelim = '}'
	case '(':
		closeDelim = ')'
	default:
		return pos, Entry{}, false
	}
	openPos := pos
	pos++

	// Special types: pass through as raw without parsing fields.
	if entryType == "comment" || entryType == "string" || entryType == "preamble" {
		end := findMatchingClose(content, openPos, content[openPos], closeDelim)
		return end, Entry{}, false
	}

	pos = skipWS(content, pos)

	// Citation key: read until comma, close delimiter, or whitespace.
	keyStart := pos
	for pos < len(content) && content[pos] != ',' && content[pos] != byte(closeDelim) && !isWS(content[pos]) {
		pos++
	}
	key := content[keyStart:pos]

	pos = skipWS(content, pos)

	if pos >= len(content) || content[pos] == byte(closeDelim) {
		if pos < len(content) {
			pos++
		}
		return pos, Entry{Type: entryType, Key: key}, true
	}
	if content[pos] == ',' {
		pos++
	}

	entry := Entry{Type: entryType, Key: key}

	for {
		pos = skipWS(content, pos)
		if pos >= len(content) || content[pos] == byte(closeDelim) {
			if pos < len(content) {
				pos++
			}
			break
		}

		// Field name
		nameStart := pos
		for pos < len(content) && content[pos] != '=' && content[pos] != byte(closeDelim) && content[pos] != ',' && !isWS(content[pos]) {
			pos++
		}
		fieldName := strings.ToLower(strings.TrimSpace(content[nameStart:pos]))

		pos = skipWS(content, pos)
		if pos >= len(content) || content[pos] == byte(closeDelim) {
			if pos < len(content) {
				pos++
			}
			break
		}
		if content[pos] == ',' {
			pos++
			continue
		}
		if content[pos] != '=' {
			for pos < len(content) && content[pos] != ',' && content[pos] != byte(closeDelim) {
				pos++
			}
			if pos < len(content) && content[pos] == ',' {
				pos++
			}
			continue
		}
		pos++ // skip '='
		pos = skipWS(content, pos)

		rawVal, newPos := readValue(content, pos)
		pos = newPos

		// Handle string concatenation with #
		pos = skipWS(content, pos)
		for pos < len(content) && content[pos] == '#' {
			pos++
			pos = skipWS(content, pos)
			extra, np := readValue(content, pos)
			pos = np
			rawVal = concatRaw(rawVal, extra)
			pos = skipWS(content, pos)
		}

		if fieldName != "" {
			entry.Fields = append(entry.Fields, Field{
				Name:  fieldName,
				Value: normalizeToBraces(rawVal),
			})
		}

		pos = skipWS(content, pos)
		if pos < len(content) && content[pos] == ',' {
			pos++
		}
	}

	return pos, entry, true
}

func readValue(s string, pos int) (string, int) {
	if pos >= len(s) {
		return "", pos
	}
	switch s[pos] {
	case '{':
		end := findMatchingClose(s, pos, '{', '}')
		return s[pos:end], end
	case '"':
		start := pos
		pos++
		depth := 0
		for pos < len(s) {
			switch s[pos] {
			case '{':
				depth++
			case '}':
				depth--
			case '"':
				if depth == 0 {
					pos++
					return s[start:pos], pos
				}
			}
			pos++
		}
		return s[start:pos], pos
	default:
		start := pos
		for pos < len(s) && s[pos] != ',' && s[pos] != '}' && s[pos] != ')' && s[pos] != '#' && s[pos] != '\n' && s[pos] != '\r' {
			pos++
		}
		return strings.TrimSpace(s[start:pos]), pos
	}
}

func normalizeToBraces(v string) string {
	if len(v) == 0 {
		return "{}"
	}
	if v[0] == '{' {
		return v
	}
	if v[0] == '"' {
		inner := v[1:]
		if len(inner) > 0 && inner[len(inner)-1] == '"' {
			inner = inner[:len(inner)-1]
		}
		return "{" + inner + "}"
	}
	return "{" + v + "}"
}

func concatRaw(a, b string) string {
	inner := stripOuterDelimiters(b)
	if len(a) >= 2 && a[0] == '{' && a[len(a)-1] == '}' {
		return a[:len(a)-1] + inner + "}"
	}
	return a + inner
}

func stripOuterDelimiters(v string) string {
	if len(v) < 2 {
		return v
	}
	if (v[0] == '{' && v[len(v)-1] == '}') || (v[0] == '"' && v[len(v)-1] == '"') {
		return v[1 : len(v)-1]
	}
	return v
}

func findMatchingClose(s string, openPos int, open, close byte) int {
	depth := 0
	for pos := openPos; pos < len(s); pos++ {
		switch s[pos] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return pos + 1
			}
		}
	}
	return len(s)
}

// FieldValue returns the inner content of a named field with outer delimiters stripped.
func FieldValue(e Entry, name string) string {
	for _, f := range e.Fields {
		if f.Name == name {
			return stripOuterDelimiters(f.Value)
		}
	}
	return ""
}

// SetField updates an existing field or appends a new one.
func SetField(e *Entry, name, value string) {
	for i, f := range e.Fields {
		if f.Name == name {
			e.Fields[i].Value = value
			return
		}
	}
	e.Fields = append(e.Fields, Field{Name: name, Value: value})
}

func isIdent(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '-'
}

func isWS(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func skipWS(s string, pos int) int {
	for pos < len(s) && isWS(s[pos]) {
		pos++
	}
	return pos
}
