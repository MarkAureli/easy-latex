package texscan

import (
	"strings"
	"testing"
)

func TestProseRuns_KeepsTextDropsCommentsMacrosBraces(t *testing.T) {
	src := `Hello world. % a comment
This \textbf{bold} text.`
	runs := ProseRuns("f.tex", src)
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
	runs := ProseRuns("f.tex", src)
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
	runs := ProseRuns("f.tex", src)
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

func TestProseRuns_NonTextMacroSkipsArgs(t *testing.T) {
	src := `See \cite{Smith2020} for details.`
	runs := ProseRuns("f.tex", src)
	if len(runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(runs))
	}
	if strings.Contains(runs[0].Text, "Smith2020") {
		t.Errorf("non-text macro arg leaked: %q", runs[0].Text)
	}
	if !strings.Contains(runs[0].Text, "details") {
		t.Errorf("trailing prose dropped: %q", runs[0].Text)
	}
}

func TestProseRuns_NonTextMacroSkipsArgsAtAnyPosition(t *testing.T) {
	src := `Pre \foo[opt]{one}{two} {three}
\bar{
  four
}
{five} post.`
	runs := ProseRuns("f.tex", src)
	var all string
	for _, r := range runs {
		all += r.Text + "\n"
	}
	for _, w := range []string{"opt", "one", "two", "three", "four", "five"} {
		if strings.Contains(all, w) {
			t.Errorf("arg %q leaked: %q", w, all)
		}
	}
	if !strings.Contains(all, "Pre") || !strings.Contains(all, "post") {
		t.Errorf("prose dropped: %q", all)
	}
}

func TestProseRuns_TextMacroKeepsArgs(t *testing.T) {
	src := `A \emph{keep} and \textcolor{red}{shown} and \multicolumn{2}{c}{cell}.`
	runs := ProseRuns("f.tex", src)
	if len(runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(runs))
	}
	got := runs[0].Text
	for _, w := range []string{"keep", "shown", "cell"} {
		if !strings.Contains(got, w) {
			t.Errorf("text arg %q dropped: %q", w, got)
		}
	}
	for _, w := range []string{"red", "2", "c}"} {
		if strings.Contains(got, w) {
			t.Errorf("skipped arg %q leaked: %q", w, got)
		}
	}
}

func TestProseRuns_ArgSkipStopsAtParagraphBreak(t *testing.T) {
	src := "\\maketitle\n\nFresh {paragraph} text."
	runs := ProseRuns("f.tex", src)
	if len(runs) != 1 || !strings.Contains(runs[0].Text, "paragraph") {
		t.Errorf("paragraph after blank line was skipped: %#v", runs)
	}
}

func TestProseRuns_ProjectMacrosOpaque(t *testing.T) {
	src := `\newcommand\todo[1]{\textbf{TODO: #1}}
\newcommand{\note}[1]{note}
\def\gap#1#2{body}
Use \todo{fix tihs} here.`
	var all string
	for _, r := range ProseRuns("f.tex", src) {
		all += r.Text + "\n"
	}
	for _, w := range []string{"TODO", "note", "body", "tihs"} {
		if strings.Contains(all, w) {
			t.Errorf("%q leaked: %q", w, all)
		}
	}
	if !strings.Contains(all, "Use") || !strings.Contains(all, "here") {
		t.Errorf("prose dropped: %q", all)
	}
}

func TestProseRuns_VerbatimEnv(t *testing.T) {
	src := `Outside.
\begin{verbatim}
recieve teh
\end{verbatim}
Done.`
	runs := ProseRuns("f.tex", src)
	for _, r := range runs {
		if strings.Contains(r.Text, "recieve") || strings.Contains(r.Text, "teh") {
			t.Errorf("verbatim leaked on line %d: %q", r.Line, r.Text)
		}
	}
}

func TestProseRuns_AtSignInMacroName(t *testing.T) {
	src := `\@oddfoot{stuff} text \foo@bar baz`
	runs := ProseRuns("f.tex", src)
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
			runs := ProseRuns("f.tex", tc.src)
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
	runs := ProseRuns("f.tex", src)
	if len(runs) != 1 || !strings.Contains(runs[0].Text, "5cmore") {
		t.Errorf("over-blanked: %q", runs[0].Text)
	}
}

func TestProseRuns_OrdinalSuffixesAfterMath(t *testing.T) {
	cases := []struct {
		name, src, leak string
	}{
		{"dollar-th", `the $n$th element`, "th"},
		{"dollar-st", `$k$st`, "st"},
		{"paren-nd", `\(m\)nd`, "nd"},
		{"bracket-rd", `\[i\]rd`, "rd"},
		{"superscript-th", `5\textsuperscript{th} time`, "th"},
		{"subscript-nd", `n\textsubscript{nd} entry`, "nd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runs := ProseRuns("f.tex", tc.src)
			for _, r := range runs {
				for w := range strings.FieldsSeq(r.Text) {
					if w == tc.leak {
						t.Errorf("ordinal %q leaked: %q", tc.leak, r.Text)
					}
				}
			}
		})
	}
}

func TestProseRuns_PreservesNonOrdinalThWords(t *testing.T) {
	// Words that contain `th` but aren't ordinal suffixes after math must
	// pass through.
	src := `the path has width and birth dates`
	runs := ProseRuns("f.tex", src)
	if len(runs) != 1 {
		t.Fatalf("want 1 run")
	}
	for _, w := range []string{"the", "path", "width", "birth"} {
		if !strings.Contains(runs[0].Text, w) {
			t.Errorf("word %q dropped: %q", w, runs[0].Text)
		}
	}
}

func TestProseRuns_LineLengthPreserved(t *testing.T) {
	src := `Hello \emph{world} here.`
	runs := ProseRuns("f.tex", src)
	if len(runs) != 1 {
		t.Fatalf("want 1 run")
	}
	if len(runs[0].Text) != len(src) {
		t.Errorf("line length not preserved: src=%d run=%d", len(src), len(runs[0].Text))
	}
}

func TestProseRunsMultilineOptArgs(t *testing.T) {
	src := "\\begin{tcolorbox}[\n    colback=white,\n    title={Box [1]}]\nHello world\n\\end{tcolorbox}\n"
	runs := ProseRuns("a.tex", src)
	var got []string
	for _, r := range runs {
		got = append(got, strings.TrimSpace(r.Text))
	}
	if len(got) != 1 || got[0] != "Hello world" {
		t.Fatalf("got %q, want only \"Hello world\"", got)
	}
}

func TestProseRunsSkipsEnvMandatoryArgs(t *testing.T) {
	src := "\\begin{tabular}{XXXXX}\nHello & world\\\\\n\\end{tabular}\n" +
		"\\begin{tabularx}{\\textwidth}{XXXXX}\nfoo\n\\end{tabularx}\n" +
		"\\begin{tabular}[t]{YYYYY}\nbar\n\\end{tabular}\n" +
		"\\begin{center}\n{\\Large Zork}\n\\end{center}\n"
	var all string
	for _, r := range ProseRuns("a.tex", src) {
		all += r.Text + "\n"
	}
	for _, bad := range []string{"XXXXX", "YYYYY", "textwidth"} {
		if strings.Contains(all, bad) {
			t.Errorf("%q leaked into prose: %q", bad, all)
		}
	}
	for _, good := range []string{"Hello", "foo", "bar", "Zork"} {
		if !strings.Contains(all, good) {
			t.Errorf("%q missing from prose: %q", good, all)
		}
	}
}
