package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
	"github.com/MarkAureli/easy-latex/internal/term"
)

// checkReport holds the pedantic and spelling results of el check / el compile.
type checkReport struct {
	fixed []pedantic.Diagnostic // removed by autofixes (--fix)
	ped   []pedantic.Diagnostic // remaining pedantic diagnostics
	spell []pedantic.Diagnostic
}

// runSourceChecks runs the source-level pedantic checks over texFiles
// (applying autofixes first when fix is set) and the spell-check.
func runSourceChecks(cfg *Config, checks, texFiles []string, fix bool) (checkReport, error) {
	var rep checkReport
	if fix {
		before := pedantic.RunSourceChecks(checks, texFiles)
		if _, err := pedantic.RunSourceFixes(checks, texFiles); err != nil {
			return rep, err
		}
		rep.ped = pedantic.RunSourceChecks(checks, texFiles)
		rep.fixed = diffDiagnostics(before, rep.ped)
	} else {
		rep.ped = pedantic.RunSourceChecks(checks, texFiles)
	}
	spell, err := runSpellCheck(cfg, texFiles)
	if err != nil {
		return rep, err
	}
	rep.spell = spell
	sortDiagnostics(rep.fixed)
	sortDiagnostics(rep.ped)
	sortDiagnostics(rep.spell)
	return rep, nil
}

// printReport prints the Pedantics (fixed), Pedantics [(remaining)],
// Misspellings and Warnings sections to stderr, separated by blank lines,
// followed by the summary. The "fixed" section uses the default colour, the
// others yellow. includeWarnings adds the compile warning count (compile only).
func printReport(rep checkReport, warns []logDiag, withFix, includeWarnings bool) {
	colors := compileColors
	first := true
	emit := func(fn func()) {
		if !first {
			fmt.Fprintln(os.Stderr)
		}
		fn()
		first = false
	}
	remainingLabel := "Pedantics"
	if withFix {
		remainingLabel = "Pedantics (remaining)"
	}
	if len(rep.fixed) > 0 {
		emit(func() { printDiagSection(os.Stderr, "Pedantics (fixed)", rep.fixed, "", "", colors) })
	}
	if len(rep.ped) > 0 {
		emit(func() { printDiagSection(os.Stderr, remainingLabel, rep.ped, colors.Yellow, colors.Yellow, colors) })
	}
	if len(rep.spell) > 0 {
		emit(func() { printDiagSection(os.Stderr, "Misspellings", rep.spell, colors.Yellow, colors.Yellow, colors) })
	}
	if len(warns) > 0 {
		emit(func() { printLogSection(os.Stderr, "Warnings:", colors.Yellow, warns) })
	}
	printSummary(os.Stderr, len(rep.ped), len(rep.spell), len(warns), includeWarnings, colors)
}

// printDiagSection prints a labelled section of diagnostics. headerColor and
// entryColor are ANSI sequences (use "" for default colour). Caller is
// responsible for blank-line separation between adjacent sections.
func printDiagSection(w *os.File, label string, diags []pedantic.Diagnostic, headerColor, entryColor string, colors term.Colors) {
	if len(diags) == 0 {
		return
	}
	fmt.Fprintf(w, "%s%s%s:%s\n", colors.Bold, headerColor, label, colors.Reset)
	for _, d := range diags {
		fmt.Fprintf(w, "  %s%s%s\n", entryColor, d.String(), colors.Reset)
	}
}

// diffDiagnostics returns the diagnostics in `before` that are not present in
// `after` (matched by File+Line+Message). Used to derive the "fixed" subset
// after running source autofixes.
func diffDiagnostics(before, after []pedantic.Diagnostic) []pedantic.Diagnostic {
	key := func(d pedantic.Diagnostic) string {
		return fmt.Sprintf("%s\x00%d\x00%s", d.File, d.Line, d.Message)
	}
	seen := make(map[string]int, len(after))
	for _, d := range after {
		seen[key(d)]++
	}
	var fixed []pedantic.Diagnostic
	for _, d := range before {
		k := key(d)
		if seen[k] > 0 {
			seen[k]--
			continue
		}
		fixed = append(fixed, d)
	}
	return fixed
}

// printSummary prints a one-line yellow summary if any count is non-zero.
// includeWarnings controls whether the compile warnings count appears (compile only).
func printSummary(w *os.File, ped, spell, warn int, includeWarnings bool, colors term.Colors) {
	if ped == 0 && spell == 0 && warn == 0 {
		return
	}
	plural := func(n int, sing, pl string) string {
		if n == 1 {
			return fmt.Sprintf("%d %s", n, sing)
		}
		return fmt.Sprintf("%d %s", n, pl)
	}
	fmt.Fprintln(w)
	parts := []string{
		plural(ped, "pedantic", "pedantics"),
		plural(spell, "misspelling", "misspellings"),
	}
	if includeWarnings {
		parts = append(parts, plural(warn, "warning", "warnings"))
	}
	summary := strings.Join(parts, ", ")
	fmt.Fprintf(w, "%s%s%s%s\n", colors.Bold, colors.Yellow, summary, colors.Reset)
}

// sortDiagnostics orders diagnostics by File then Line for stable output.
func sortDiagnostics(d []pedantic.Diagnostic) {
	sort.SliceStable(d, func(i, j int) bool {
		if d[i].File != d[j].File {
			return d[i].File < d[j].File
		}
		return d[i].Line < d[j].Line
	})
}
