package pedantic

import "testing"

func TestDashes_Fixer_Unicode(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"a–b", "a---b"},              // en-dash, then rule 8 promotes to em
		{"a—b", "a---b"},              // em-dash → ---
		{"value − 5", "value $-$ 5"},  // unicode minus in text → $-$
		{"$− 5$", "$- 5$"},            // unicode minus in math → -
	}
	for _, c := range cases {
		out, _ := fixDashes("t.tex", []string{c.in})
		if out[0] != c.want {
			t.Errorf("input %q: got %q, want %q", c.in, out[0], c.want)
		}
	}
}

func TestDashes_Fixer_NumericRanges(t *testing.T) {
	cases := []struct{ in, want string }{
		{"pp. 10-20", "pp. 10--20"},
		{"pp. 10 - 20", "pp. 10--20"},
		{"pp. 10---20", "pp. 10--20"},
		{"pp. 10 --- 20", "pp. 10--20"},
		{"years 1939-1945", "years 1939--1945"},
		{"a 1-2-3 b", "a 1--2--3 b"},
	}
	for _, c := range cases {
		out, _ := fixDashes("t.tex", []string{c.in})
		if out[0] != c.want {
			t.Errorf("input %q: got %q, want %q", c.in, out[0], c.want)
		}
	}
}

func TestDashes_Fixer_HyphenCollapse(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a ---- b", "a---b"}, // 4 hy collapses, then spaces strip
		{"a ----- b", "a---b"},
		{"a -------- b", "a---b"},
	}
	for _, c := range cases {
		out, _ := fixDashes("t.tex", []string{c.in})
		if out[0] != c.want {
			t.Errorf("input %q: got %q, want %q", c.in, out[0], c.want)
		}
	}
}

func TestDashes_Fixer_EmDashSpacing(t *testing.T) {
	cases := []struct{ in, want string }{
		{"foo --- bar", "foo---bar"},
		{"foo ---bar", "foo---bar"},
		{"foo--- bar", "foo---bar"},
		{"foo---bar", "foo---bar"},
	}
	for _, c := range cases {
		out, _ := fixDashes("t.tex", []string{c.in})
		if out[0] != c.want {
			t.Errorf("input %q: got %q, want %q", c.in, out[0], c.want)
		}
	}
}

func TestDashes_Fixer_EnDashBetweenWords(t *testing.T) {
	cases := []struct{ in, want string }{
		{"foo--bar", "foo---bar"},     // tight en between lower → em
		{"foo -- bar", "foo---bar"},   // spaced en between lower → em
		{"Foo--Bar", "Foo--Bar"},      // both cap → keep
		{"Foo -- Bar", "Foo -- Bar"},  // both cap spaced → keep
		{"foo--Bar", "foo---Bar"},     // mixed → convert
		{"Foo--bar", "Foo---bar"},     // mixed → convert
		{"a -- b -- c", "a---b---c"},  // chained
	}
	for _, c := range cases {
		out, _ := fixDashes("t.tex", []string{c.in})
		if out[0] != c.want {
			t.Errorf("input %q: got %q, want %q", c.in, out[0], c.want)
		}
	}
}

func TestDashes_Fixer_SpacedHyphenWords(t *testing.T) {
	cases := []struct{ in, want string }{
		{"foo - bar", "foo---bar"},     // spaced hy → em
		{"Foo - Bar", "Foo---Bar"},     // both cap still converts (Q2)
		{"a - b - c", "a---b---c"},     // chained
		{"well-known", "well-known"},   // tight hyphen kept
		{"mother-in-law", "mother-in-law"},
	}
	for _, c := range cases {
		out, _ := fixDashes("t.tex", []string{c.in})
		if out[0] != c.want {
			t.Errorf("input %q: got %q, want %q", c.in, out[0], c.want)
		}
	}
}

func TestDashes_Fixer_SkipMath(t *testing.T) {
	// In math mode, none of the text rules fire. Rule 3a (unicode minus → -)
	// does fire there. Plain `-` and `--` should be left alone.
	cases := []struct{ in, want string }{
		{"$a - b$", "$a - b$"},
		{"$1-2$", "$1-2$"},
		{"$a--b$", "$a--b$"},
		{"$a − b$", "$a - b$"},
		{"\\(x - y\\)", "\\(x - y\\)"},
	}
	for _, c := range cases {
		out, _ := fixDashes("t.tex", []string{c.in})
		if out[0] != c.want {
			t.Errorf("input %q: got %q, want %q", c.in, out[0], c.want)
		}
	}
}

func TestDashes_Fixer_SkipVerbatim(t *testing.T) {
	in := []string{
		"\\begin{verbatim}",
		"foo - bar 1-2 — baz",
		"\\end{verbatim}",
	}
	out, changed := fixDashes("t.tex", append([]string(nil), in...))
	if changed {
		t.Errorf("verbatim modified: %v", out)
	}
}

func TestDashes_Fixer_SkipComment(t *testing.T) {
	// Comment tail should be preserved verbatim, even if it contains dashes
	// that would otherwise trigger rules.
	in := []string{"foo - bar % see 10-20 in pp."}
	want := []string{"foo---bar % see 10-20 in pp."}
	out, changed := fixDashes("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected change")
	}
	if out[0] != want[0] {
		t.Errorf("got %q, want %q", out[0], want[0])
	}
}

func TestDashes_Detector(t *testing.T) {
	lines := []string{
		"clean line",
		"foo -- bar",      // violation
		"$a - b$",         // ok (math)
		"pp. 10-20",       // violation
	}
	diags := checkDashes("t.tex", lines)
	if len(diags) != 2 {
		t.Fatalf("got %d diags, want 2: %v", len(diags), diags)
	}
	if diags[0].Line != 2 || diags[1].Line != 4 {
		t.Errorf("diag lines = %d, %d; want 2, 4", diags[0].Line, diags[1].Line)
	}
}

func TestDashes_Fixer_SkipMacroArgs(t *testing.T) {
	cases := []struct{ in, want string }{
		{"\\documentclass{revtex4-2}", "\\documentclass{revtex4-2}"},
		{"\\documentclass[10pt]{revtex4-2}", "\\documentclass[10pt]{revtex4-2}"},
		{"\\usepackage{foo-bar2-3}", "\\usepackage{foo-bar2-3}"},
		{"\\WarningFilter{revtex4-2}{Repeated}", "\\WarningFilter{revtex4-2}{Repeated}"},
		{"\\input{chap1-2/intro}", "\\input{chap1-2/intro}"},
		// rule still fires outside the skip range
		{"\\documentclass{revtex4-2} pp. 10-20", "\\documentclass{revtex4-2} pp. 10--20"},
	}
	for _, c := range cases {
		out, _ := fixDashes("t.tex", []string{c.in})
		if out[0] != c.want {
			t.Errorf("input %q: got %q, want %q", c.in, out[0], c.want)
		}
	}
}

func TestDashes_NoOpClean(t *testing.T) {
	in := []string{
		"well-known compound",
		"mother-in-law",
		"$x - y$",
		"% comment with - dashes",
		"Foo--Bar relation",
		"\\begin{verbatim}",
		"raw -- text 1-2",
		"\\end{verbatim}",
	}
	_, changed := fixDashes("t.tex", append([]string(nil), in...))
	if changed {
		t.Error("expected no change for clean input")
	}
}
