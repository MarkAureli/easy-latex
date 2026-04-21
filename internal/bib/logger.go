package bib

// Logger receives diagnostic and progress messages from bib operations.
type Logger interface {
	// Info prints an informational message (success, status).
	Info(key, msg string)
	// Warn prints a warning.
	Warn(key, msg string)
	// Progress prints a transient status (e.g., "Fetching from Crossref...").
	Progress(key, msg string)
}

// logOrNop returns log if non-nil, otherwise a nopLogger.
func logOrNop(log Logger) Logger {
	if log != nil {
		return log
	}
	return nopLogger{}
}

// nopLogger discards all messages.
type nopLogger struct{}

func (nopLogger) Info(string, string)     {}
func (nopLogger) Warn(string, string)     {}
func (nopLogger) Progress(string, string) {}

