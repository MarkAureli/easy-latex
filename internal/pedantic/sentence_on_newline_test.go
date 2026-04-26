package pedantic

import (
	"reflect"
	"testing"
)

func TestSentenceOnNewline_Detector(t *testing.T) {
	lines := []string{
		`Foo. Bar baz.`,                // line 1: violation
		`We use Mr. Smith and Dr. Who.`, // line 2: abbrevs → no flag
		`Number 1. Foo`,                 // line 3: digit-only word → no flag
		`See e.g. Foo`,                  // line 4: abbrev `e.g` → no flag
		`End of line.`,                  // line 5: sentence ends EOL → ok
		`Foo? Then Bar!`,                // line 6: ? → flag (capital after)
		`Foo. bar continues`,            // line 7: lowercase after → no flag
	}
	diags := checkSentenceOnNewline("t.tex", lines)
	wantLines := []int{1, 6}
	if len(diags) != len(wantLines) {
		t.Fatalf("got %d diags, want %d: %+v", len(diags), len(wantLines), diags)
	}
	for i, want := range wantLines {
		if diags[i].Line != want {
			t.Errorf("diag[%d].Line = %d, want %d (msg=%q)", i, diags[i].Line, want, diags[i].Message)
		}
	}
}

func TestSentenceOnNewline_MathSkipped(t *testing.T) {
	lines := []string{
		`Inline $f(x). G(x)$ stays inline`, // mid-math sentence → skip
		`Display \[ a. B \] outside`,        // mid-display sentence → skip
	}
	diags := checkSentenceOnNewline("t.tex", lines)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags inside math, got %d: %+v", len(diags), diags)
	}
}

func TestSentenceOnNewline_VerbatimSkipped(t *testing.T) {
	lines := []string{
		`\begin{verbatim}`,
		`Foo. Bar would normally flag`,
		`\end{verbatim}`,
	}
	diags := checkSentenceOnNewline("t.tex", lines)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags inside verbatim, got %d: %+v", len(diags), diags)
	}
}

func TestSentenceOnNewline_Fixer(t *testing.T) {
	in := []string{
		`Foo. Bar baz.`,
		`  Indented. Next sentence here.`,
		`See e.g. Foo and Mr. Smith. Done.`,
	}
	want := []string{
		`Foo.`,
		`Bar baz.`,
		`  Indented.`,
		`  Next sentence here.`,
		`See e.g. Foo and Mr. Smith.`,
		`Done.`,
	}
	out, changed := fixSentenceOnNewline("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changed=true")
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("output mismatch:\ngot:  %q\nwant: %q", out, want)
	}
}

func TestSentenceOnNewline_FixerPreservesComment(t *testing.T) {
	in := []string{
		`Foo. Bar baz. % tail`,
	}
	want := []string{
		`Foo.`,
		`Bar baz. % tail`,
	}
	out, changed := fixSentenceOnNewline("t.tex", append([]string(nil), in...))
	if !changed {
		t.Fatal("expected changed=true")
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got:  %q\nwant: %q", out, want)
	}
}
