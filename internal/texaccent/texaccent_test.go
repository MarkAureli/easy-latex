package texaccent

import "testing"

func TestExpand_AllForms(t *testing.T) {
	accent := func(cmd, letter string) (string, bool) {
		return "<" + cmd + letter + ">", true
	}
	stand := func(cmd, b string) (string, bool) { return "[" + cmd + "]" + b, true }

	cases := []struct {
		in, want string
	}{
		{`\'e`, `<'e>`},
		{`\'{e}`, `<'e>`},
		{`{\'e}`, `<'e>`},
		{`{\'{e}}`, `<'e>`},
		{`\"a`, `<"a>`},
		{`\v{c}`, `<vc>`},
		{`\u{a}`, `<ua>`},
		{`\ss `, `[ss] `},
		{`\aa.`, `[aa].`},
		{`\o`, `[o]`},
		{`\AE!`, `[AE]!`},
	}
	for _, c := range cases {
		got := Expand(c.in, accent, stand)
		if got != c.want {
			t.Errorf("Expand(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExpand_NoFalseAlpha(t *testing.T) {
	// `\url`, `\vspace`, `\Huge`, `\centering`, `\textbf` must NOT be matched
	// as alpha accents (they share the leading letter with u/v/H/c/t).
	accent := func(cmd, letter string) (string, bool) { return "<HIT>", true }
	stand := func(cmd, b string) (string, bool) { return "<HIT>", true }
	for _, in := range []string{`\url{x}`, `\vspace{1cm}`, `\Huge{x}`, `\centering`, `\textbf{x}`} {
		if got := Expand(in, accent, stand); got != in {
			t.Errorf("Expand(%q) = %q, expected untouched", in, got)
		}
	}
}
