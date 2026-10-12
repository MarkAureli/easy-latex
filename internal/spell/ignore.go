package spell

import (
	"bufio"
	"os"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// LoadIgnoreMacros returns the set of macro names listed in the given ignore
// files (global first, then local). A listed macro is removed from the text
// whitelist (see TextMacros), so all of its arguments are skipped. Files that
// do not exist are skipped. Blank lines, `#` comments and legacy `!negation`
// lines are ignored.
func LoadIgnoreMacros(files ...string) map[string]bool {
	set := map[string]bool{}
	for _, path := range files {
		applyIgnoreFile(set, path)
	}
	return set
}

// TextMacros builds the effective text-macro whitelist for ProseRuns:
// texscan.DefaultTextMacros plus every macro defined in the project (value 0:
// all brace args are prose), minus the macros in ignore.
func TextMacros(defined []string, ignore map[string]bool) map[string]int {
	out := make(map[string]int, len(texscan.DefaultTextMacros)+len(defined))
	for _, m := range defined {
		out[m] = 0
	}
	for m, skip := range texscan.DefaultTextMacros {
		out[m] = skip
	}
	for m := range ignore {
		delete(out, m)
	}
	return out
}

func applyIgnoreFile(set map[string]bool, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "!") {
			continue
		}
		set[line] = true
	}
}
