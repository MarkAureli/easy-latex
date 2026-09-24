package bib

import "testing"

func TestCollapsePageRange(t *testing.T) {
	tests := []struct{ in, want string }{
		{"331-331", "331"},
		{"1--1", "1"},
		{"12–12", "12"},
		{"100-110", "100-110"},
		{"042612", "042612"},
		{"e1-e1", "e1"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := collapsePageRange(tc.in); got != tc.want {
			t.Errorf("collapsePageRange(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
