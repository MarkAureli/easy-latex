package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
)

// appendDiagsToLog appends pedantic and spelling diagnostics to the engine log
// as `Package el Warning: ... on input line N.` so VimTeX's latexlog parser
// lists them in :VimtexErrors as warnings. (`file:line:` form would parse as an
// error and make VimTeX report the compilation as failed.) The parser cannot
// attach a file to warnings, so the file name is part of the message and the
// entry's location falls back to the main file.
func appendDiagsToLog(logPath string, ped, spell []pedantic.Diagnostic) error {
	var b strings.Builder
	b.WriteString("\n")
	emitLinelessBoxes(&b, logPath)
	if len(ped)+len(spell) == 0 && b.Len() == 1 {
		return nil
	}
	emit := func(tag string, diags []pedantic.Diagnostic) {
		for _, d := range diags {
			line := max(d.Line, 1)
			msg := strings.ReplaceAll(d.Message, "\n", " ")
			fmt.Fprintf(&b, "Package el Warning: %s: [%s] %s on input line %d.\n\n", d.File, tag, msg, line)
		}
	}
	emit("el", ped)
	emit("el:spell", spell)

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(b.String())
	return err
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
