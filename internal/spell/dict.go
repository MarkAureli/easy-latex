package spell

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

// MergeDicts reads all given dict files (lang + common, global + local) and
// writes a merged personal-dict file at outPath in hunspell `.dic` format:
// first line = entry count, subsequent lines = one word each. Missing input
// files are silently skipped. Blank lines and `#` comments are ignored. Words
// are deduplicated and sorted for determinism. Returns the number of words
// written.
func MergeDicts(outPath string, files ...string) (int, error) {
	words := map[string]struct{}{}
	for _, path := range files {
		readDict(path, words)
	}
	keys := make([]string, 0, len(words))
	for w := range words {
		keys = append(keys, w)
	}
	sort.Strings(keys)

	f, err := os.Create(outPath)
	if err != nil {
		return 0, fmt.Errorf("create %s: %w", outPath, err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "%d\n", len(keys))
	for _, k := range keys {
		fmt.Fprintln(w, k)
	}
	return len(keys), w.Flush()
}

func readDict(path string, into map[string]struct{}) {
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
		into[line] = struct{}{}
	}
}
