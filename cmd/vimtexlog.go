package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
)

// appendDiagsToLog appends pedantic and spelling diagnostics to the engine log
// in `file:line: message` form (like -file-line-error), so VimTeX's latexlog
// parser lists them in :VimtexErrors with the correct source file. (Its
// LaTeX Warning entries cannot carry a file; they fall back to the main file.)
func appendDiagsToLog(logPath string, ped, spell []pedantic.Diagnostic) error {
	if len(ped)+len(spell) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("\n")
	emit := func(tag string, diags []pedantic.Diagnostic) {
		for _, d := range diags {
			line := max(d.Line, 1)
			msg := strings.ReplaceAll(d.Message, "\n", " ")
			fmt.Fprintf(&b, "%s:%d: [%s] %s\n\n", d.File, line, tag, msg)
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
