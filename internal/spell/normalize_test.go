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
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			content := NormalizeAccents(NormalizeSharpS(tc.src))
			runs := texscan.ProseRuns("f.tex", content, nil)
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

func TestNormalizeSharpS(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"literal-eszett", "Bußmann", "Bussmann"},
		{"capital-eszett", "BUẞMANN", "BUSSMANN"},
		{"ss-braces", `Bu\ss{}mann`, "Bussmann"},
		{"ss-group", `Bu{\ss}mann`, "Bussmann"},
		{"ss-group-braces", `Bu{\ss{}}mann`, "Bussmann"},
		{"ss-space-letter", `Bu\ss mann`, "Bussmann"},
		{"ss-punct", `foo \ss.`, "foo ss."},
		{"ss-end", `Bu\ss`, "Buss"},
		{"SS-braces", `BU\SS{}MANN`, "BUSSMANN"},
		{"SS-group", `BU{\SS}MANN`, "BUSSMANN"},
		{"no-touch-longer-macro", `\ssfoo`, `\ssfoo`},
		{"no-touch-letter-followed", `Bu\ssX`, `Bu\ssX`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeSharpS(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeSharpS(%q) = %q, want %q", tc.in, got, tc.want)
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
		{"no-touch-non-vowel", `\"x`, `\"x`},
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
		// standalone letter commands
		{"aa", `\aa ngstr\"om`, "ångström"},
		{"AE", `\AE on`, "Æon"},
		{"oe", `\oe uvre`, "œuvre"},
		{"o-slash", `\o re`, "øre"},
		{"l", `\l odz`, "łodz"},
		{"dotless-i", `\i j`, "ıj"},
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
