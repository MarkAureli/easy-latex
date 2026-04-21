package term

import (
	"os"

	xterm "golang.org/x/term"
)

// Colors holds ANSI escape sequences, empty when not a terminal.
type Colors struct {
	Reset  string
	Red    string
	Yellow string
	Green  string
	Bold   string
	Dim    string
}

// Detect returns Colors populated with ANSI codes if stderr is a terminal,
// or empty strings otherwise.
func Detect() Colors {
	if !IsTerminal() {
		return Colors{}
	}
	return Colors{
		Reset:  "\033[0m",
		Red:    "\033[31m",
		Yellow: "\033[33m",
		Green:  "\033[32m",
		Bold:   "\033[1m",
		Dim:    "\033[2m",
	}
}

// Width returns the terminal column count, or 80 if detection fails.
func Width() int {
	w, _, err := xterm.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// IsTerminal reports whether stdout is a terminal.
func IsTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
