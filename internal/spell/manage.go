package spell

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// ResolveTarget returns the dict/ignore file path implied by flag combination.
// isIgnore and isCommon are mutually exclusive (caller must validate). When
// isIgnore=false and isCommon=false, lang is required and selects the
// per-language dict; otherwise lang is ignored.
func ResolveTarget(globalDir, auxDir, lang string, isGlobal, isIgnore, isCommon bool) (string, error) {
	base := auxDir
	if isGlobal {
		base = globalDir
	}
	switch {
	case isIgnore:
		return filepath.Join(base, "spell", "ignore.txt"), nil
	case isCommon:
		return filepath.Join(base, "spell", "common.txt"), nil
	default:
		if lang == "" {
			return "", fmt.Errorf("no spelling language configured (run `el config set spelling en_GB|en_US`, or use --common / --ignore)")
		}
		return filepath.Join(base, "spell", lang+".txt"), nil
	}
}

// ValidateToken rejects tokens with whitespace. Empty and pure-whitespace
// tokens are rejected too.
func ValidateToken(tok string) error {
	if tok == "" {
		return fmt.Errorf("empty token")
	}
	for _, r := range tok {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return fmt.Errorf("token %q contains whitespace", tok)
		}
	}
	return nil
}

// AddTokens appends tokens to path, then rewrites the file as sorted-unique.
// Returns the number of tokens that were not previously present (i.e. truly
// added). Legacy `!token` negation lines left over from the old default-ignore
// scheme are kept untouched.
func AddTokens(path string, tokens []string) (int, error) {
	for _, t := range tokens {
		if err := ValidateToken(t); err != nil {
			return 0, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return 0, err
	}
	lines, err := readLinesIfExists(path)
	if err != nil {
		return 0, err
	}
	set := map[string]bool{}
	for _, l := range lines {
		set[l] = true
	}
	added := 0
	for _, t := range tokens {
		if set[t] {
			continue
		}
		set[t] = true
		added++
	}
	return added, writeLinesSorted(path, set)
}

// RemoveTokens deletes any line of path matching a token. Returns the number
// of tokens that were present and removed.
func RemoveTokens(path string, tokens []string) (int, error) {
	for _, t := range tokens {
		if err := ValidateToken(t); err != nil {
			return 0, err
		}
	}
	lines, err := readLinesIfExists(path)
	if err != nil {
		return 0, err
	}
	set := map[string]bool{}
	for _, l := range lines {
		set[l] = true
	}
	removed := 0
	for _, t := range tokens {
		if set[t] {
			delete(set, t)
			removed++
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return removed, err
	}
	return removed, writeLinesSorted(path, set)
}

// ListTokens returns the sorted-unique non-comment, non-blank lines of path.
// Missing file returns nil, nil.
func ListTokens(path string) ([]string, error) {
	lines, err := readLinesIfExists(path)
	if err != nil {
		return nil, err
	}
	out := slices.Clone(lines)
	sort.Strings(out)
	return out, nil
}

// CompletionCandidates returns the sorted-unique lines of path, suitable for
// shell completion of `el spell remove`.
func CompletionCandidates(path string) []string {
	lines, _ := readLinesIfExists(path)
	set := map[string]bool{}
	for _, l := range lines {
		set[l] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// MisspellingsPath returns the canonical path of the per-project misspellings
// cache file. Callers pass the project's aux dir.
func MisspellingsPath(auxDir string) string {
	return filepath.Join(auxDir, "spell", "misspellings.txt")
}

// WriteMisspellings rewrites path with words, sorted-unique. Empty words
// truncates the file. Creates parent dirs.
func WriteMisspellings(path string, words []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	set := map[string]bool{}
	for _, w := range words {
		if w == "" {
			continue
		}
		set[w] = true
	}
	return writeLinesSorted(path, set)
}

// LoadMisspellings returns the sorted-unique words recorded at path. Missing
// file returns nil.
func LoadMisspellings(path string) []string {
	lines, _ := readLinesIfExists(path)
	out := slices.Clone(lines)
	sort.Strings(out)
	return out
}

// RemoveMisspellings drops the given tokens from path. Missing file or missing
// tokens are no-ops.
func RemoveMisspellings(path string, tokens []string) error {
	lines, err := readLinesIfExists(path)
	if err != nil || len(lines) == 0 {
		return err
	}
	drop := map[string]bool{}
	for _, t := range tokens {
		drop[t] = true
	}
	set := map[string]bool{}
	for _, l := range lines {
		if !drop[l] {
			set[l] = true
		}
	}
	return writeLinesSorted(path, set)
}

// readLinesIfExists returns non-blank, non-comment trimmed lines, or nil if the
// file does not exist.
func readLinesIfExists(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		s := strings.TrimSpace(sc.Text())
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		out = append(out, s)
	}
	return out, sc.Err()
}

// writeLinesSorted rewrites path with the sorted-unique entries of set, one
// per line. Negation entries (`!foo`) are sorted with the rest. Empty set
// truncates the file.
func writeLinesSorted(path string, set map[string]bool) error {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, k := range keys {
		fmt.Fprintln(w, k)
	}
	return w.Flush()
}
