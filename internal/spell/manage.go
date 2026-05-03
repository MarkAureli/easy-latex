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
// added). For ignore files, adding a token that is already covered by a
// `DefaultIgnoreMacros` entry or already present results in no-op for that
// token; if the file contains a `!token` negation line, that line is dropped
// (un-negate) instead of appending.
func AddTokens(path string, tokens []string, isIgnore bool) (int, error) {
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
	defaults := map[string]bool{}
	if isIgnore {
		for _, m := range DefaultIgnoreMacros {
			defaults[m] = true
		}
	}
	added := 0
	for _, t := range tokens {
		if isIgnore {
			neg := "!" + t
			if set[neg] {
				delete(set, neg)
				added++
				continue
			}
			if defaults[t] || set[t] {
				continue
			}
			set[t] = true
			added++
			continue
		}
		if set[t] {
			continue
		}
		set[t] = true
		added++
	}
	return added, writeLinesSorted(path, set)
}

// RemoveTokens removes tokens from path. For dict files: deletes any matching
// line. For ignore files: if the token is a `DefaultIgnoreMacros` entry and not
// already negated, writes `!token` (negate default); if a user-added line
// matches, deletes it; both may apply (user added a duplicate of a default).
// Returns (removedFromFile, negatedDefaults).
func RemoveTokens(path string, tokens []string, isIgnore bool) (int, int, error) {
	for _, t := range tokens {
		if err := ValidateToken(t); err != nil {
			return 0, 0, err
		}
	}
	lines, err := readLinesIfExists(path)
	if err != nil {
		return 0, 0, err
	}
	set := map[string]bool{}
	for _, l := range lines {
		set[l] = true
	}
	defaults := map[string]bool{}
	if isIgnore {
		for _, m := range DefaultIgnoreMacros {
			defaults[m] = true
		}
	}
	removed, negated := 0, 0
	for _, t := range tokens {
		userLine := set[t]
		if userLine {
			delete(set, t)
			removed++
		}
		if isIgnore && defaults[t] {
			neg := "!" + t
			if !set[neg] {
				set[neg] = true
				negated++
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return removed, negated, err
	}
	return removed, negated, writeLinesSorted(path, set)
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

// CompletionCandidates returns sort-unique entries suitable for shell
// completion of `el spell remove`. For ignore targets, the union of file
// entries and DefaultIgnoreMacros is returned (so users can negate defaults).
func CompletionCandidates(path string, isIgnore bool) []string {
	lines, _ := readLinesIfExists(path)
	set := map[string]bool{}
	for _, l := range lines {
		// Strip leading `!` so "!cite" surfaces as completable "cite".
		set[strings.TrimPrefix(l, "!")] = true
	}
	if isIgnore {
		for _, m := range DefaultIgnoreMacros {
			set[m] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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
