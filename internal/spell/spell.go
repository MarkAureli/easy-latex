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
// callers can convert without importing this package into pedantic.
type Diagnostic struct {
	File    string
	Line    int
	Message string
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

// Run spell-checks all files (path → comment-stripped or raw lines — we use
// raw text reassembled from lines via "\n" join) in the given lang. auxDir is
// the project `.el` directory used for the temp personal-dict file.
//
// Returns nil if hunspell or its dict for lang is unavailable (warn-once).
func Run(files map[string][]string, lang, auxDir string, paths Paths, warn io.Writer) []Diagnostic {
	if !HunspellAvailable(lang, warn) {
		return nil
	}

	// Build personal dict from layered sources.
	if err := os.MkdirAll(filepath.Join(auxDir, "spell"), 0755); err != nil {
		fmt.Fprintf(warn, "warning: spell-check: cannot create %s/spell: %v\n", auxDir, err)
		return nil
	}
	personalPath := filepath.Join(auxDir, "spell", "personal-"+lang+".dic")
	if _, err := MergeDicts(personalPath,
		paths.GlobalCommonDict, paths.GlobalLangDict,
		paths.LocalCommonDict, paths.LocalLangDict,
	); err != nil {
		fmt.Fprintf(warn, "warning: spell-check: cannot merge dicts: %v\n", err)
		return nil
	}

	ignoreSet := LoadIgnoreMacros(paths.GlobalIgnore, paths.LocalIgnore)

	hs, err := StartHunspell(lang, personalPath)
	if err != nil {
		hunspellMissingWarned.Do(func() {
			fmt.Fprintf(warn, "warning: spell-check skipped: cannot start hunspell for %q (dictionary likely missing): %v\n", lang, err)
		})
		return nil
	}
	defer hs.Close()

	// Stable iteration order over files for deterministic diagnostics.
	paths2 := make([]string, 0, len(files))
	for p := range files {
		paths2 = append(paths2, p)
	}
	sort.Strings(paths2)

	var out []Diagnostic
	for _, path := range paths2 {
		content := strings.Join(files[path], "\n")
		content = NormalizeUmlauts(NormalizeSharpS(content))
		runs := texscan.ProseRuns(path, content, ignoreSet)
		for _, r := range runs {
			misses, err := hs.CheckLine(r.Text)
			if err != nil {
				fmt.Fprintf(warn, "warning: spell-check: hunspell pipe broken: %v\n", err)
				return out
			}
			for _, m := range misses {
				msg := fmt.Sprintf("spelling: %q [%s] col %d", m.Word, lang, m.Col)
				if len(m.Suggestions) > 0 {
					show := m.Suggestions
					if len(show) > 5 {
						show = show[:5]
					}
					msg = fmt.Sprintf("%s (suggest: %s)", msg, strings.Join(show, ", "))
				}
				out = append(out, Diagnostic{File: r.File, Line: r.Line, Message: msg})
			}
		}
	}
	return out
}
