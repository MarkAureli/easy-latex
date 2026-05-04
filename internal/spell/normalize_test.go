package spell

import "testing"

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
