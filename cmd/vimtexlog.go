package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
)

// appendDiagsToLog prepares the engine log for VimTeX's latexlog parser:
//
//   - every warning in the log is preceded by a `**"file"` line naming the
//     source file it came from. VimTeX's errorformat treats `**file` as a
//     file-stack push (`%-P**\"%f\"`), so the following warning entry gets that
//     file instead of the main-file fallback. Without this, only
//     Over/Underfull boxes in space-free file names get a correct file.
//   - pedantic and spelling diagnostics are appended as
//     `Package el Warning: ... on input line N.` (a `file:line:` form would
//     parse as an error and make VimTeX report the compilation as failed),
//     each preceded by its own `**"file"` push.
//   - lineless Over/Underfull boxes are re-emitted (see emitLinelessBoxes),
//     attributed to mainFile since the source location is unknown.
func appendDiagsToLog(logPath, mainFile string, ped, spell []pedantic.Diagnostic) error {
	orig, err := os.ReadFile(logPath)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(annotateLogFiles(string(orig), mainFile))
	b.WriteString("\n")
	b.WriteString(logFilePush(mainFile))
	emitLinelessBoxes(&b, logPath)
	emit := func(tag string, diags []pedantic.Diagnostic) {
		for _, d := range diags {
			line := max(d.Line, 1)
			msg := strings.ReplaceAll(d.Message, "\n", " ")
			b.WriteString(logFilePush(d.File))
			fmt.Fprintf(&b, "Package el Warning: %s: [%s] %s on input line %d.\n\n", d.File, tag, msg, line)
		}
	}
	emit("el", ped)
	emit("el:spell", spell)
	return os.WriteFile(logPath, []byte(b.String()), 0644)
}

const boxDecoyLine = "(el-unknown-file)\n"

// logFilePush returns the `**"/abs/file"` file-stack push line VimTeX understands.
func logFilePush(file string) string {
	// Absolute: nvim resolves quickfix paths against its own cwd, which need not
	// be the project root (el runs with the main file's directory as cwd).
	if abs, err := filepath.Abs(file); err == nil {
		file = abs
	}
	return "**\"" + file + "\"\n"
}

// annotateLogFiles inserts a file-stack push line before every diagnostic in
// the engine log, naming the project file TeX was reading at that point
// (mainFile when unknown).
func annotateLogFiles(log, mainFile string) string {
	var tr logFileTracker
	var b strings.Builder
	for _, l := range strings.SplitAfter(log, "\n") {
		text := strings.TrimSuffix(l, "\n")
		isDiag := !contextLinePattern.MatchString(text) && isDiagnosticLine(text)
		if isDiag && !tr.inBlock && !strings.HasPrefix(text, "Package el Warning") {
			f := tr.current()
			if f == "" {
				f = mainFile
			}
			b.WriteString(logFilePush(f))
			if strings.HasPrefix(text, "Overfull") || strings.HasPrefix(text, "Underfull") {
				// VimTeX re-resolves the file of box warnings by scanning upwards
				// for a line ending in "(file", which mis-attributes names with
				// spaces. A decoy unreadable file on the line directly above makes
				// that lookup give up and keep the pushed file.
				b.WriteString(boxDecoyLine)
			}
		}
		tr.feed(text, isDiag)
		b.WriteString(l)
	}
	return b.String()
}

// emitLinelessBoxes re-emits `Underfull|Overfull \vbox|\hbox ... has occurred
// while \output is active` log lines as `Package el Warning`, because VimTeX's
// errorformat only matches boxes with an `at line N` and silently drops these.
func emitLinelessBoxes(b *strings.Builder, logPath string) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return
	}
	for _, l := range strings.Split(string(data), "\n") {
		if !strings.Contains(l, "while \\output is active") {
			continue
		}
		for _, kind := range []string{"Underfull \\vbox", "Overfull \\vbox", "Underfull \\hbox", "Overfull \\hbox"} {
			if !strings.HasPrefix(l, kind) {
				continue
			}
			fmt.Fprintf(b, "Package el Warning: [%s] %s on input line 1.\n\n", kind[len(kind)-4:], strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(l), "[]")))
			break
		}
	}
}
