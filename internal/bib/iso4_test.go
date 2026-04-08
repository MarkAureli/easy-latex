package bib

import "testing"

func TestAbbreviateISO4(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		// Single-word titles are never abbreviated.
		{"Nature", "Nature"},
		{"Science", "Science"},
		{"Gut", "Gut"},

		// Common multi-word patterns.
		{"Nature Communications", "Nat. Commun."},
		{"Journal of Applied Chemistry", "J. Appl. Chem."},
		{"Journal of the American Chemical Society", "J. Am. Chem. Soc."},
		{"Physical Review Letters", "Phys. Rev. Lett."},
		{"Proceedings of the National Academy of Sciences", "Proc. Natl. Acad. Sci."},

		// Leading article stripped.
		{"The Lancet", "Lancet"},

		// Trailing parenthetical stripped before single-word check.
		{"Lancet (London)", "Lancet"},
		{"Nature (London)", "Nature"},

		// Trailing parenthetical stripped before abbreviation.
		{"Nature Communications (Print)", "Nat. Commun."},

		// Hyphenated words: each segment abbreviated independently.
		// Single-word rule applies to whitespace tokens, so a standalone
		// hyphenated title is not abbreviated.
		{"Bio-Chemistry", "Bio-Chemistry"},
		{"Journal of Bio-Chemistry", "J. Bio-Chem."},
	}

	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			got := AbbreviateISO4(tc.title)
			if got != tc.want {
				t.Errorf("AbbreviateISO4(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}
