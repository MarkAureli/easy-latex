package cmd

import (
	"fmt"
	"os"

	"github.com/MarkAureli/easy-latex/internal/bib"
	"github.com/MarkAureli/easy-latex/internal/term"
)

// bibLogger implements bib.Logger with colored terminal output.
type bibLogger struct {
	colors term.Colors
}

func newBibLogger() bib.Logger {
	return &bibLogger{colors: term.Detect()}
}

func (l *bibLogger) Info(key, msg string) {
	if key != "" {
		fmt.Printf("[bib] %s%s%s: %s\n", l.colors.Bold, key, l.colors.Reset, msg)
	} else {
		fmt.Printf("[bib] %s\n", msg)
	}
}

func (l *bibLogger) Warn(key, msg string) {
	prefix := "[bib] "
	if key != "" {
		prefix += key + ": "
	}
	fmt.Fprintf(os.Stderr, "%s%s%s%s\n", l.colors.Yellow, prefix, msg, l.colors.Reset)
}

func (l *bibLogger) Progress(key, msg string) {
	if key != "" {
		fmt.Fprintf(os.Stderr, "%s[bib] %s: %s%s\n", l.colors.Dim, key, msg, l.colors.Reset)
	} else {
		fmt.Fprintf(os.Stderr, "%s[bib] %s%s\n", l.colors.Dim, msg, l.colors.Reset)
	}
}
