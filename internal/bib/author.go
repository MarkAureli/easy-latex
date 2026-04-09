package bib

import (
	"strings"
	"unicode"
)

// formatAuthorField normalises the value of an author field.
//
// Authors are separated by " and " (brace-aware). Each individual author is
// normalised to "Last, F. M." form. Author tokens already wrapped in braces
// (e.g. {Google Quantum AI}) are treated as organisations and left unchanged.
//
// If maxAuthors > 0 and there are more authors than that limit, only the first
// maxAuthors authors are kept and "others" is appended, causing BibTeX/BibLaTeX
// to render "et al.".
func formatAuthorField(authors string, maxAuthors int) string {
	parts := splitByAnd(authors)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, formatSingleAuthor(strings.TrimSpace(p)))
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
// Otherwise the token is normalised to "Last, F. M." form, handling both
// "Last, First Middle" and "First Middle Last" input.
func formatSingleAuthor(name string) string {
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

	abbrev := abbreviateGivenNames(givens)
	if abbrev == "" {
		return last
	}
	return last + ", " + abbrev
}

// abbreviateGivenNames returns space-separated initials for a list of given
// names, e.g. "John Frank" → "J. F.".
func abbreviateGivenNames(givens string) string {
	parts := strings.Fields(givens)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if init := initialOf(p); init != "" {
			out = append(out, init)
		}
	}
	return strings.Join(out, " ")
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
