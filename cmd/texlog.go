package cmd

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
)

var errorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^!`),
	// Anchored: a bare "error" word also appears in paragraph text echoed as
	// Over/Underfull hbox context and must not turn warnings into errors.
	regexp.MustCompile(`\b(?:Error:|error \()`),
	regexp.MustCompile(`^\(There were \d+ errors?\)`),
	regexp.MustCompile(`^(?:ERROR|FATAL) - `), // biber
	regexp.MustCompile(`(?i)\bfatal error\b`),
}

var warningPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)warning`),
	regexp.MustCompile(`^WARN - `), // biber
	regexp.MustCompile(`(?i)undefined`),
	regexp.MustCompile(`(?i)multiply defined`),
	regexp.MustCompile(`^(?:Over|Under)full`),
}

// contextLinePattern matches TeX's "l.N ..." error context line.
var contextLinePattern = regexp.MustCompile(`^l\.\d+`)

// "may have changed" catches natbib's two-line rerun signal where the
// continuation "(natbib) Rerun to get them right." is not a diagnostic.
var rerunPattern = regexp.MustCompile(`(?i)rerun|may have changed`)

type logKind uint8

const (
	logNone logKind = iota
	logWarning
	logError
)

// classifyLine reports whether line starts a diagnostic and of which kind.
// Error patterns take precedence; context lines are never diagnostics.
func classifyLine(line string) logKind {
	if contextLinePattern.MatchString(line) {
		return logNone
	}
	for _, pat := range errorPatterns {
		if pat.MatchString(line) {
			return logError
		}
	}
	for _, pat := range warningPatterns {
		if pat.MatchString(line) {
			return logWarning
		}
	}
	return logNone
}

// logDiag is one error or warning from engine or bib tool output.
type logDiag struct {
	Kind    logKind
	File    string   // source file TeX was reading (warnings only; errors carry it via -file-line-error)
	Text    string   // the diagnostic line
	Context []string // "l.N ..." lines that followed it
}

// String renders the diagnostic line, naming its source file if known.
func (d logDiag) String() string {
	if d.File != "" {
		return d.File + ": " + d.Text
	}
	return d.Text
}

// parseLog extracts all errors and warnings from TeX or bib tool output.
func parseLog(output []byte) []logDiag {
	var diags []logDiag
	var files logFileTracker
	for line := range strings.SplitSeq(string(output), "\n") {
		// Biber's progress lines, incl. the "INFO - WARNINGS: n" summary, are noise.
		if strings.HasPrefix(line, "INFO - ") {
			continue
		}
		kind := classifyLine(line)
		curFile := files.current()
		files.feed(line, kind != logNone)
		switch {
		case contextLinePattern.MatchString(line):
			if len(diags) > 0 {
				last := &diags[len(diags)-1]
				last.Context = append(last.Context, line)
			}
		case kind == logError:
			diags = append(diags, logDiag{Kind: logError, Text: line})
		case kind == logWarning:
			// -file-line-error already prefixes errors; warnings only carry
			// "input line N", so name the file.
			diags = append(diags, logDiag{Kind: logWarning, File: curFile, Text: line})
		}
	}
	return diags
}

// ofKind returns the diagnostics of kind k.
func ofKind(diags []logDiag, k logKind) []logDiag {
	var out []logDiag
	for _, d := range diags {
		if d.Kind == k {
			out = append(out, d)
		}
	}
	return out
}

// needsRerun reports whether a diagnostic asks for another engine pass.
func needsRerun(diags []logDiag) bool {
	return slices.ContainsFunc(diags, func(d logDiag) bool {
		return rerunPattern.MatchString(d.Text)
	})
}

// printLogDiags prints the errors in diags, or its warnings if there are no
// errors, under an "Error:" or "Warnings:" header.
func printLogDiags(w io.Writer, diags []logDiag) {
	if errs := ofKind(diags, logError); len(errs) > 0 {
		printLogSection(w, "Error:", compileColors.Red, errs)
		return
	}
	printLogSection(w, "Warnings:", compileColors.Yellow, diags)
}

// printLogSection prints diags under header, separated by blank lines, each
// followed by its context lines.
func printLogSection(w io.Writer, header, color string, diags []logDiag) {
	if len(diags) == 0 {
		return
	}
	fmt.Fprintf(w, "%s%s%s%s\n", compileColors.Bold, color, header, compileColors.Reset)
	for i, d := range diags {
		if i > 0 {
			fmt.Fprintln(w)
		}
		for _, line := range append([]string{d.String()}, d.Context...) {
			fmt.Fprintf(w, "  %s%s%s\n", color, line, compileColors.Reset)
		}
	}
}
