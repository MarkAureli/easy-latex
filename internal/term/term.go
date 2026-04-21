package term

import "os"

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

// IsTerminal reports whether stdout is a terminal.
func IsTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
