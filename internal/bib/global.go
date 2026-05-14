package bib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// GlobalBibPath returns the path to the global bib cache file. Resolution
// order:
//
//  1. $EL_GLOBAL_BIB (full path to bib.json) if set
//  2. $XDG_DATA_HOME/easy-latex/bib.json if set
//  3. ~/Library/Application Support/easy-latex/bib.json on darwin
//  4. ~/.local/share/easy-latex/bib.json on other unix-like systems
//
// The parent directory is created on demand by writers; callers may receive a
// path whose directory does not yet exist.
func GlobalBibPath() (string, error) {
	if p := os.Getenv("EL_GLOBAL_BIB"); p != "" {
		return p, nil
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "easy-latex", "bib.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "easy-latex", "bib.json"), nil
	}
	return filepath.Join(home, ".local", "share", "easy-latex", "bib.json"), nil
}

// ensureGlobalDir creates the parent directory of the global cache file if
// missing. Returns the resolved path.
func ensureGlobalDir() (string, error) {
	p, err := GlobalBibPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return "", fmt.Errorf("could not create %s: %w", filepath.Dir(p), err)
	}
	return p, nil
}
