package pedantic

import (
	"io"
	"os"

	"github.com/MarkAureli/easy-latex/internal/spell"
)

// Spelling configuration. Set by callers via ConfigureSpelling before
// RunSourceChecks. When SpellLang is empty the registered `spelling` check is
// a no-op even if listed in the enabled set.
var (
	spellLang      string
	spellGlobalDir string
	spellAuxDir    string
	spellWarn      io.Writer = os.Stderr
)

// ConfigureSpelling configures the `spelling` pedantic check. lang in
// {"", "en_GB", "en_US"} — empty disables. globalDir is the easy-latex global
// config directory (for global dicts/ignore). auxDir is the project `.el`
// directory (for local dicts/ignore and the temp personal dict).
func ConfigureSpelling(lang, globalDir, auxDir string) {
	spellLang = lang
	spellGlobalDir = globalDir
	spellAuxDir = auxDir
}

// SetSpellingWarn overrides the writer for spell-check warnings (default:
// os.Stderr). Useful in tests.
func SetSpellingWarn(w io.Writer) { spellWarn = w }

func init() {
	Register(Check{
		Name:  "spelling",
		Phase: PhaseProjectSource,
		ProjectSource: func(files map[string][]string) []Diagnostic {
			if spellLang == "" {
				return nil
			}
			paths := spell.DefaultPaths(spellGlobalDir, spellAuxDir, spellLang)
			diags := spell.Run(files, spellLang, spellAuxDir, paths, spellWarn)
			out := make([]Diagnostic, len(diags))
			for i, d := range diags {
				out[i] = Diagnostic{File: d.File, Line: d.Line, Message: d.Message}
			}
			return out
		},
	})
}
