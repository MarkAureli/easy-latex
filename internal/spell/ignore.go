package spell

import (
	"bufio"
	"os"
	"strings"
)

// DefaultIgnoreMacros lists tex macros whose first brace argument should be
// skipped during spell-checking (citations, refs, file paths, package names,
// URLs, image paths, …). Always-on; extensible via global/local ignore.txt
// (additive). To remove an entry, prefix with `!` in an ignore.txt line.
var DefaultIgnoreMacros = []string{
	// Citation
	"cite", "citep", "citet", "citeauthor", "citeyear", "citeyearpar",
	"parencite", "textcite", "autocite", "fullcite", "footcite", "smartcite",
	"Cite", "Citep", "Citet", "Citeauthor", "Parencite", "Textcite", "Autocite",
	// References / labels
	"ref", "Ref", "eqref", "autoref", "Autoref",
	"cref", "Cref", "crefrange", "Crefrange",
	"labelcref", "pageref", "nameref", "vref", "Vref",
	"vpageref", "autopageref", "hyperref", "label",
	// URLs / files / packages
	"url", "href", "nolinkurl", "urlstyle",
	"input", "include", "includeonly", "InputIfFileExists", "IfFileExists",
	"bibliography", "addbibresource", "bibliographystyle",
	"usepackage", "RequirePackage", "documentclass", "LoadClass",
	"PassOptionsToClass", "PassOptionsToPackage", "WarningFilter",
	"ProvidesPackage", "ProvidesClass", "ProvidesFile",
	"includegraphics", "graphicspath",
	// Counters / commands often arg-only
	"setcounter", "addtocounter", "stepcounter", "refstepcounter",
	"newcounter", "newcommand", "renewcommand", "providecommand",
	"DeclareMathOperator", "newenvironment", "renewenvironment",
	"NewDocumentCommand", "RenewDocumentCommand", "ProvideDocumentCommand",
	"NewDocumentEnvironment", "RenewDocumentEnvironment",
}

// LoadIgnoreMacros builds the active ignore set: code-baked defaults, plus
// additive entries from the given files (later files apply on top of earlier),
// minus entries prefixed with `!` in any of those files. Files that do not
// exist are skipped. Lines starting with `#` and blank lines are ignored.
func LoadIgnoreMacros(files ...string) map[string]bool {
	set := make(map[string]bool, len(DefaultIgnoreMacros)*2)
	for _, m := range DefaultIgnoreMacros {
		set[m] = true
	}
	for _, path := range files {
		applyIgnoreFile(set, path)
	}
	return set
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
		if rest, ok := strings.CutPrefix(line, "!"); ok {
			delete(set, rest)
			continue
		}
		set[line] = true
	}
}
