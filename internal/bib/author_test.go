package bib

import "testing"

func TestFormatAuthorField(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Already-abbreviated single author
		{"already abbreviated", "Smith, J. F.", "Smith, J. F."},
		// Full given names — comma form
		{"comma form full names", "Smith, John Frank", "Smith, J. F."},
		// Full given names — natural order
		{"natural order", "John Frank Smith", "Smith, J. F."},
		// Single given name
		{"single given", "Smith, John", "Smith, J."},
		// No given name
		{"no given", "Smith", "Smith"},
		// Multiple authors
		{"multiple authors", "Smith, John and Doe, Jane", "Smith, J. and Doe, J."},
		// Three authors
		{"three authors", "Einstein, Albert and Bohr, Niels and Curie, Marie",
			"Einstein, A. and Bohr, N. and Curie, M."},
		// Organisation in braces — kept unchanged
		{"organisation", "{Google Quantum AI}", "{Google Quantum AI}"},
		// Mixed personal and organisation
		{"mixed", "Smith, John and {OpenAI}", "Smith, J. and {OpenAI}"},
		// arXiv-style natural order
		{"arxiv style", "John Smith and Jane Doe", "Smith, J. and Doe, J."},
		// Hyphenated first name — initial is first letter
		{"hyphenated first", "Jean-Pierre Dupont", "Dupont, J."},
		// LaTeX accent in given name
		{"latex accent given", `Smith, {\'E}tienne`, "Smith, E."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAuthorField(tc.in, 0, true)
			if got != tc.want {
				t.Errorf("formatAuthorField(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatAuthorFieldFullFirstName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"full first, middle abbreviated", "Smith, John Frank", "Smith, John F."},
		{"natural order full first", "John Frank Smith", "Smith, John F."},
		{"single given name kept full", "Smith, John", "Smith, John"},
		{"no given name", "Smith", "Smith"},
		{"already initial stays", "Smith, J. F.", "Smith, J. F."},
		{"multiple authors", "Smith, John Frank and Doe, Jane Mary", "Smith, John F. and Doe, Jane M."},
		{"organisation unchanged", "{Google Quantum AI}", "{Google Quantum AI}"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAuthorField(tc.in, 0, false)
			if got != tc.want {
				t.Errorf("formatAuthorField(%q, 0, false) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatAuthorFieldMaxAuthors(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		maxAuthors int
		want       string
	}{
		{"unlimited", "Smith, John and Doe, Jane and Lee, Bob", 0, "Smith, J. and Doe, J. and Lee, B."},
		{"limit 1 of 3", "Smith, John and Doe, Jane and Lee, Bob", 1, "Smith, J. and others"},
		{"limit 2 of 3", "Smith, John and Doe, Jane and Lee, Bob", 2, "Smith, J. and Doe, J. and others"},
		{"limit equals count", "Smith, John and Doe, Jane", 2, "Smith, J. and Doe, J."},
		{"limit exceeds count", "Smith, John and Doe, Jane", 5, "Smith, J. and Doe, J."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAuthorField(tc.in, tc.maxAuthors, true)
			if got != tc.want {
				t.Errorf("formatAuthorField(%q, %d) = %q, want %q", tc.in, tc.maxAuthors, got, tc.want)
			}
		})
	}
}

func TestSplitByAnd(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"Smith and Doe", []string{"Smith", "Doe"}},
		{"Smith", []string{"Smith"}},
		{"{and co} and Doe", []string{"{and co}", "Doe"}},
		{"Smith and Doe and Lee", []string{"Smith", "Doe", "Lee"}},
	}
	for _, tc := range tests {
		got := splitByAnd(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("splitByAnd(%q): got %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitByAnd(%q)[%d]: got %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestInitialOf(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"John", "J."},
		{"J.", "J."},
		{"J", "J."},
		{"", ""},
		{`{\'E}tienne`, "E."},
		{"Jean-Pierre", "J."},
	}
	for _, tc := range tests {
		got := initialOf(tc.in)
		if got != tc.want {
			t.Errorf("initialOf(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
