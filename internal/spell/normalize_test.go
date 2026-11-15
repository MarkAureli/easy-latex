package spell

import (
	"strings"
	"testing"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// Regression for the original `Ren\'{e}` → `Reneeeee` bug: after normalize,
// ProseRuns must keep the accented word as a single contiguous token.
func TestNormalizeAccents_ProsePipeline(t *testing.T) {
	cases := []struct {
		src, wantContains string
	}{
		{`Ren\'{e}`, "René"},
		{`Universit\"at`, "Universität"},
		{`Bo\u{g}azi\c{c}i`, "Boğaziçi"},
		{`\aa ngstr\"om`, "ångström"},
		{`Bu\ss mann`, "Bußmann"},
		{`Stra{\ss}e`, "Straße"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			content := NormalizeAccents(tc.src)
			runs := texscan.ProseRuns("f.tex", content)
			if len(runs) == 0 {
				t.Fatalf("no prose runs for %q (normalized=%q)", tc.src, content)
			}
			text := strings.Join(func() []string {
				out := make([]string, len(runs))
				for i, r := range runs {
					out[i] = r.Text
				}
				return out
			}(), " ")
			if !strings.Contains(text, tc.wantContains) {
				t.Errorf("ProseRuns(%q) = %q, want to contain %q", content, text, tc.wantContains)
			}
		})
	}
}

func TestNormalizeAccents(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		// umlauts
		{"umlaut-bare", `f\"ur`, "für"},
		{"umlaut-braces", `f\"{u}r`, "für"},
		{"umlaut-group", `f{\"u}r`, "für"},
		{"umlaut-group-braces", `f{\"{u}}r`, "für"},
		{"umlaut-capital", `\"Uber`, "Über"},
		{"umlaut-multiple", `\"u\"o\"a`, "üöä"},
		{"umlaut-already-utf8", "für", "für"},
		{"diaeresis-x-combining", `\"x`, "x\u0308"},
		// acute (the user-reported case)
		{"acute-braces", `Ren\'{e}`, "René"},
		{"acute-bare", `Ren\'e`, "René"},
		{"acute-group", `Ren{\'e}`, "René"},
		{"acute-group-braces", `Ren{\'{e}}`, "René"},
		// grave / circumflex / tilde / macron
		{"grave", `\` + "`" + `a`, "à"},
		{"circumflex", `\^o`, "ô"},
		{"tilde-n", `ma\~nana`, "mañana"},
		{"macron", `\=a`, "ā"},
		// alpha-letter accents
		{"caron", `\v{c}`, "č"},
		{"breve", `Bo\u{g}azi`, "Boğazi"},
		{"cedilla", `gar\c{c}on`, "garçon"},
		{"double-acute", `Erd\H{o}s`, "Erdős"},
		// no precomposed form: letter + combining mark
		{"breve-c-combining", `Mare\u{c}ek`, "Marec\u0306ek"},
		{"breve-C-combining", `\u{C}`, "C\u0306"},
		{"acute-q-combining", `\'q`, "q\u0301"},
		// standalone letter commands
		{"aa", `\aa ngstr\"om`, "ångström"},
		{"AE", `\AE on`, "Æon"},
		{"oe", `\oe uvre`, "œuvre"},
		{"o-slash", `\o re`, "øre"},
		{"l", `\l odz`, "łodz"},
		{"dotless-i", `\i j`, "ıj"},
		// sharp-s: now maps to ß for consistency with other accents
		{"ss-space", `Bu\ss mann`, "Bußmann"},
		{"ss-punct", `foo \ss.`, "foo ß."},
		{"ss-group", `Bu{\ss}mann`, "Bußmann"},
		{"ss-group-empty-arg", `Bu{\ss{}}mann`, "Bußmann"},
		{"ss-empty-arg", `Bu\ss{}mann`, "Bußmann"},
		{"SS-group", `BU{\SS}MANN`, "BUẞMANN"},
		{"raw-eszett-untouched", "Bußmann", "Bußmann"},
		{"no-touch-longer-macro", `\ssfoo`, `\ssfoo`},
		// must not match longer macro names
		{"no-touch-url", `\url{x}`, `\url{x}`},
		{"no-touch-vspace", `\vspace{1cm}`, `\vspace{1cm}`},
		{"no-touch-Huge", `\Huge text`, `\Huge text`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeAccents(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeAccents(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
