package texscan

import (
	"strings"
	"testing"
)

func TestProseRuns_KeepsTextDropsCommentsMacrosBraces(t *testing.T) {
	src := `Hello world. % a comment
This \textbf{bold} text.`
	runs := ProseRuns("f.tex", src, nil)
	if len(runs) != 2 {
		t.Fatalf("want 2 runs, got %d: %#v", len(runs), runs)
	}
	if !strings.Contains(runs[0].Text, "Hello world.") {
		t.Errorf("run0 missing prose: %q", runs[0].Text)
	}
	if strings.Contains(runs[0].Text, "comment") {
		t.Errorf("run0 leaked comment: %q", runs[0].Text)
	}
	if strings.Contains(runs[1].Text, "textbf") {
		t.Errorf("run1 leaked macro name: %q", runs[1].Text)
	}
	if !strings.Contains(runs[1].Text, "bold") {
		t.Errorf("run1 dropped arg text: %q", runs[1].Text)
	}
	// Column preservation: "world" in run0 starts at col 7 in source.
	if idx := strings.Index(runs[0].Text, "world"); idx != 6 {
		t.Errorf("col preservation broken: want offset 6, got %d in %q", idx, runs[0].Text)
	}
}

func TestProseRuns_BlanksMath(t *testing.T) {
	src := `Let $x = y$ be the root.`
	runs := ProseRuns("f.tex", src, nil)
	if len(runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(runs))
	}
	if strings.Contains(runs[0].Text, "x = y") {
		t.Errorf("math content leaked: %q", runs[0].Text)
	}
	if !strings.Contains(runs[0].Text, "Let") || !strings.Contains(runs[0].Text, "root") {
		t.Errorf("prose dropped: %q", runs[0].Text)
	}
}

func TestProseRuns_BlanksDisplayMathEnv(t *testing.T) {
	src := `Before.
\begin{equation}
  E = mc^2
\end{equation}
After.`
	runs := ProseRuns("f.tex", src, nil)
	// Lines 1 and 5 carry prose; lines 2-4 are math env (no prose bytes).
	var prose []string
	for _, r := range runs {
		if s := strings.TrimSpace(r.Text); s != "" {
			prose = append(prose, s)
		}
	}
	if len(prose) != 2 || prose[0] != "Before." || prose[1] != "After." {
		t.Errorf("unexpected prose: %#v", prose)
	}
}

func TestProseRuns_IgnoreMacroArg(t *testing.T) {
	src := `See \cite{Smith2020} for details.`
	ign := map[string]bool{"cite": true}
	runs := ProseRuns("f.tex", src, ign)
	if len(runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(runs))
	}
	if strings.Contains(runs[0].Text, "Smith2020") {
		t.Errorf("ignored macro arg leaked: %q", runs[0].Text)
	}
	if !strings.Contains(runs[0].Text, "details") {
		t.Errorf("trailing prose dropped: %q", runs[0].Text)
	}
}

func TestProseRuns_IgnoreMacroArgMultiline(t *testing.T) {
	src := `Pre.
\bibliography{
  refs1,
  refs2
}
Post.`
	ign := map[string]bool{"bibliography": true}
	runs := ProseRuns("f.tex", src, ign)
	for _, r := range runs {
		if strings.Contains(r.Text, "refs1") || strings.Contains(r.Text, "refs2") {
			t.Errorf("multiline arg leaked on line %d: %q", r.Line, r.Text)
		}
	}
}

func TestProseRuns_VerbatimEnv(t *testing.T) {
	src := `Outside.
\begin{verbatim}
recieve teh
\end{verbatim}
Done.`
	runs := ProseRuns("f.tex", src, nil)
	for _, r := range runs {
		if strings.Contains(r.Text, "recieve") || strings.Contains(r.Text, "teh") {
			t.Errorf("verbatim leaked on line %d: %q", r.Line, r.Text)
		}
	}
}

func TestProseRuns_AccentMacroDoesNotSplitWord(t *testing.T) {
	cases := []struct {
		name, src string
	}{
		{"bare", `Universit\"at`},
		{"bare-braces", `Universit\"{a}t`},
		{"wrapped", `Universit{\"a}t`},
		{"wrapped-double", `Universit{\"{a}}t`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runs := ProseRuns("f.tex", tc.src, nil)
			if len(runs) != 1 {
				t.Fatalf("want 1 run, got %d", len(runs))
			}
			text := runs[0].Text
			if len(text) != len(tc.src) {
				t.Errorf("length not preserved: src=%d got=%d", len(tc.src), len(text))
			}
			// No internal whitespace inside the token region (positions 0 to len-1).
			if strings.ContainsAny(strings.TrimSpace(text), " \t") {
				t.Errorf("accent split word: %q", text)
			}
		})
	}
}

func TestProseRuns_AtSignInMacroName(t *testing.T) {
	src := `\@oddfoot{stuff} text \foo@bar baz`
	runs := ProseRuns("f.tex", src, nil)
	if len(runs) != 1 {
		t.Fatalf("want 1 run")
	}
	for _, leaked := range []string{"oddfoot", "foo", "bar"} {
		if strings.Contains(runs[0].Text, leaked) {
			t.Errorf("@-macro name leaked %q in %q", leaked, runs[0].Text)
		}
	}
	if !strings.Contains(runs[0].Text, "text") || !strings.Contains(runs[0].Text, "baz") {
		t.Errorf("surrounding prose lost: %q", runs[0].Text)
	}
}

func TestProseRuns_BlanksLengthLiterals(t *testing.T) {
	cases := []struct {
		name, src, leak string
	}{
		{"cm", `width 8cm here`, "8cm"},
		{"mm", `5mm spacing`, "5mm"},
		{"ex", `15ex tall`, "15ex"},
		{"em", `2em wide`, "2em"},
		{"pt", `12pt font`, "12pt"},
		{"decimal", `2.5cm spacing`, "2.5cm"},
		{"negative", `-3pt offset`, "-3pt"},
		{"fill", `5fill stretch`, "5fill"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runs := ProseRuns("f.tex", tc.src, nil)
			if len(runs) != 1 {
				t.Fatalf("want 1 run")
			}
			if strings.Contains(runs[0].Text, tc.leak) {
				t.Errorf("length literal leaked: %q", runs[0].Text)
			}
		})
	}
}

func TestProseRuns_PreservesNonLengthIdentifiers(t *testing.T) {
	// "5cmore" must NOT be blanked; only the boundary-terminated form is a length.
	src := `keep 5cmore please`
	runs := ProseRuns("f.tex", src, nil)
	if len(runs) != 1 || !strings.Contains(runs[0].Text, "5cmore") {
		t.Errorf("over-blanked: %q", runs[0].Text)
	}
}

func TestProseRuns_LineLengthPreserved(t *testing.T) {
	src := `Hello \emph{world} here.`
	runs := ProseRuns("f.tex", src, nil)
	if len(runs) != 1 {
		t.Fatalf("want 1 run")
	}
	if len(runs[0].Text) != len(src) {
		t.Errorf("line length not preserved: src=%d run=%d", len(src), len(runs[0].Text))
	}
}
