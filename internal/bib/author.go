package bib

import (
	"strings"
	"unicode"
)

// formatAuthorField normalises the value of an author field.
//
// Authors are separated by " and " (brace-aware). Each individual author is
// normalised to "Last, F. M." form when abbreviateFirstName is true, or
// "Last, First M." form when false. Author tokens already wrapped in braces
// (e.g. {Google Quantum AI}) are treated as organisations and left unchanged.
//
// If maxAuthors > 0 and there are more authors than that limit, only the first
// maxAuthors authors are kept and "others" is appended, causing BibTeX/BibLaTeX
// to render "et al.".
func formatAuthorField(authors string, maxAuthors int, abbreviateFirstName bool) string {
	parts := splitByAnd(authors)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, formatSingleAuthor(strings.TrimSpace(p), abbreviateFirstName))
	}
	if maxAuthors > 0 && len(out) > maxAuthors {
		out = append(out[:maxAuthors], "others")
	}
	return strings.Join(out, " and ")
}

// splitByAnd splits s on " and " at brace depth 0.
func splitByAnd(s string) []string {
	const sep = " and "
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '{':
			depth++
			i++
		case '}':
			depth--
			i++
		default:
			if depth == 0 && strings.HasPrefix(s[i:], sep) {
				parts = append(parts, s[start:i])
				i += len(sep)
				start = i
			} else {
				i++
			}
		}
	}
	return append(parts, s[start:])
}

// formatSingleAuthor normalises one author token.
//
// Tokens already wrapped in braces are returned unchanged (organisations).
// Otherwise the token is normalised to "Last, F. M." form when
// abbreviateFirstName is true, or "Last, First M." form when false (first
// given name kept in full, middle names still abbreviated). Both "Last, First
// Middle" and "First Middle Last" input forms are handled.
func formatSingleAuthor(name string, abbreviateFirstName bool) string {
	if strings.HasPrefix(name, "{") && strings.HasSuffix(name, "}") {
		return name // organisation — keep as-is
	}

	var last, givens string
	if l, g, found := strings.Cut(name, ","); found {
		last = strings.TrimSpace(l)
		givens = strings.TrimSpace(g)
	} else {
		parts := strings.Fields(name)
		switch len(parts) {
		case 0:
			return name
		case 1:
			return name // single token, nothing to do
		default:
			last = parts[len(parts)-1]
			givens = strings.Join(parts[:len(parts)-1], " ")
		}
	}

	abbrev := abbreviateGivenNames(givens, abbreviateFirstName)
	if abbrev == "" {
		return last
	}
	return last + ", " + abbrev
}

// abbreviateGivenNames returns formatted given names.
//
// When abbreviateFirstName is true all names are reduced to initials, e.g.
// "John Frank" → "J. F.".
// When false the first name is kept verbatim and only middle names are
// abbreviated, e.g. "John Frank" → "John F.".
func abbreviateGivenNames(givens string, abbreviateFirstName bool) string {
	parts := strings.Fields(givens)
	out := make([]string, 0, len(parts))
	for i, p := range parts {
		if i == 0 && !abbreviateFirstName {
			out = append(out, p)
		} else {
			if init := initialOf(p); init != "" {
				out = append(out, init)
			}
		}
	}
	return strings.Join(out, " ")
}

// normalizeAllCapsName converts a fully uppercase name to title case.
// If any letter in name is lowercase, the string is returned unchanged.
// Word boundaries are detected at spaces, hyphens, and apostrophes so that
// "JEAN-PIERRE" → "Jean-Pierre" and "O'BRIAN" → "O'Brian".
func normalizeAllCapsName(name string) string {
	hasLetter := false
	for _, r := range name {
		if unicode.IsLetter(r) {
			hasLetter = true
			if unicode.IsLower(r) {
				return name
			}
		}
	}
	if !hasLetter {
		return name
	}
	var buf strings.Builder
	buf.Grow(len(name))
	afterSep := true // start of string counts as separator
	for _, r := range name {
		if unicode.IsLetter(r) {
			if afterSep {
				buf.WriteRune(r) // keep uppercase
			} else {
				buf.WriteRune(unicode.ToLower(r))
			}
			afterSep = false
		} else {
			buf.WriteRune(r)
			afterSep = r == ' ' || r == '-' || r == '\''
		}
	}
	return buf.String()
}

// initialOf returns the abbreviated initial for a single name part.
//
// Names already in abbreviated form ("J.") are returned unchanged.
// For all other inputs the first Unicode letter is extracted, e.g.
// "John" → "J.", "{\'E}tienne" → "E.".
func initialOf(name string) string {
	if name == "" {
		return ""
	}
	// Already an initial: one or a few chars ending with "."
	if len(name) <= 3 && strings.HasSuffix(name, ".") {
		return name
	}
	// Find the first Unicode letter (skips LaTeX control chars, braces, etc.)
	for _, ch := range name {
		if unicode.IsLetter(ch) {
			return string(ch) + "."
		}
	}
	return ""
}
