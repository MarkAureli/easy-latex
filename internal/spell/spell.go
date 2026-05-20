package spell

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// Diagnostic is a spell-check finding. Mirrors pedantic.Diagnostic shape so
// callers can convert without importing this package into pedantic. Col is
// 1-based byte column of Word's first byte on the normalized line; Len is
// Word's byte length. Suggestions carries hunspell's near-miss list (may be
// empty); callers may render or ignore it.
type Diagnostic struct {
	File        string
	Line        int
	Col         int
	Word        string
	Message     string
	Suggestions []string
}

// Paths bundles dict and ignore-file locations to consult. Missing files are
// silently skipped at load time.
type Paths struct {
	GlobalLangDict   string
	GlobalCommonDict string
	LocalLangDict    string
	LocalCommonDict  string
	GlobalIgnore     string
	LocalIgnore      string
}

// DefaultPaths returns the conventional dict/ignore paths for a given lang.
// globalDir is typically the easy-latex global config dir; auxDir is the
// project's `.el` directory.
func DefaultPaths(globalDir, auxDir, lang string) Paths {
	return Paths{
		GlobalLangDict:   filepath.Join(globalDir, "spell", lang+".txt"),
		GlobalCommonDict: filepath.Join(globalDir, "spell", "common.txt"),
		LocalLangDict:    filepath.Join(auxDir, "spell", lang+".txt"),
		LocalCommonDict:  filepath.Join(auxDir, "spell", "common.txt"),
		GlobalIgnore:     filepath.Join(globalDir, "spell", "ignore.txt"),
		LocalIgnore:      filepath.Join(auxDir, "spell", "ignore.txt"),
	}
}

// Session is a long-lived spell-check handle that owns a running hunspell
// subprocess and a loaded ignore-macros set. Cheap to call Check repeatedly;
// avoids the spawn-per-call cost incurred by Run. Callers MUST Close when done.
type Session struct {
	hs        *Hunspell
	ignoreSet map[string]bool
	warn      io.Writer
}

// NewSession launches hunspell and loads the layered ignore lists. Returns
// (nil, nil) — with a warn-once message — when hunspell or its dict for lang
// is unavailable. Returns a non-nil error only on filesystem failures the
// caller should treat as fatal.
func NewSession(lang, auxDir string, paths Paths, warn io.Writer) (*Session, error) {
	if !HunspellAvailable(lang, warn) {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Join(auxDir, "spell"), 0755); err != nil {
		return nil, fmt.Errorf("spell: cannot create %s/spell: %w", auxDir, err)
	}
	personalPath := filepath.Join(auxDir, "spell", "personal-"+lang+".dic")
	if _, err := MergeDicts(personalPath,
		paths.GlobalCommonDict, paths.GlobalLangDict,
		paths.LocalCommonDict, paths.LocalLangDict,
	); err != nil {
		return nil, fmt.Errorf("spell: cannot merge dicts: %w", err)
	}
	hs, err := StartHunspell(lang, personalPath)
	if err != nil {
		hunspellMissingWarned.Do(func() {
			fmt.Fprintf(warn, "warning: spell-check skipped: cannot start hunspell for %q (dictionary likely missing): %v\n", lang, err)
		})
		return nil, nil
	}
	return &Session{
		hs:        hs,
		ignoreSet: LoadIgnoreMacros(paths.GlobalIgnore, paths.LocalIgnore),
		warn:      warn,
	}, nil
}

// Close releases the hunspell subprocess.
func (s *Session) Close() error {
	if s == nil || s.hs == nil {
		return nil
	}
	return s.hs.Close()
}

// Check spell-checks the given files (path → raw lines). Files are joined,
// accents normalised, prose extracted, then each prose line checked.
// Diagnostics use stable file order.
func (s *Session) Check(files map[string][]string) []Diagnostic {
	if s == nil {
		return nil
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var out []Diagnostic
	for _, path := range paths {
		content := NormalizeAccents(strings.Join(files[path], "\n"))
		runs := texscan.ProseRuns(path, content, s.ignoreSet)
		for _, r := range runs {
			misses, err := s.hs.CheckLine(r.Text)
			if err != nil {
				fmt.Fprintf(s.warn, "warning: spell-check: hunspell pipe broken: %v\n", err)
				return out
			}
			for _, m := range misses {
				msg := fmt.Sprintf("%q col %d", m.Word, m.Col)
				if len(m.Suggestions) > 0 {
					show := m.Suggestions
					if len(show) > 5 {
						show = show[:5]
					}
					msg = fmt.Sprintf("%s (suggest: %s)", msg, strings.Join(show, ", "))
				}
				out = append(out, Diagnostic{
					File:        r.File,
					Line:        r.Line,
					Col:         m.Col,
					Word:        m.Word,
					Message:     msg,
					Suggestions: m.Suggestions,
				})
			}
		}
	}
	return out
}

// Run is a convenience wrapper: open a Session, check, close. For one-shot
// CLI use (el check, el compile). For long-running processes (el lsp), use
// NewSession directly.
//
// Returns nil if hunspell or its dict for lang is unavailable (warn-once).
func Run(files map[string][]string, lang, auxDir string, paths Paths, warn io.Writer) []Diagnostic {
	sess, err := NewSession(lang, auxDir, paths, warn)
	if err != nil {
		fmt.Fprintf(warn, "warning: spell-check: %v\n", err)
		return nil
	}
	if sess == nil {
		return nil
	}
	defer sess.Close()
	return sess.Check(files)
}
