package pedantic

import (
	"fmt"
	"sort"
	"strings"
)

// Diagnostic represents a single pedantic check violation.
type Diagnostic struct {
	File    string
	Line    int
	Message string
}

// String formats as "file:line: message" or "file: message" if line is 0.
func (d Diagnostic) String() string {
	if d.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", d.File, d.Line, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.File, d.Message)
}

// Phase indicates when a check runs.
type Phase int

const (
	PhaseSource Phase = iota // runs on tex source
)

// SourceCheckFunc checks source lines (comment-stripped) for a single file.
type SourceCheckFunc func(path string, lines []string) []Diagnostic

// Check describes a registered pedantic check.
type Check struct {
	Name   string
	Phase  Phase
	Source SourceCheckFunc
}

var registry = map[string]Check{}

// Register adds a check to the global registry.
func Register(c Check) {
	registry[c.Name] = c
}

// Known returns true if name is a registered check.
func Known(name string) bool {
	_, ok := registry[name]
	return ok
}

// AllNames returns sorted list of all registered check names.
func AllNames() []string {
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Get returns the check for the given name.
func Get(name string) (Check, bool) {
	c, ok := registry[name]
	return c, ok
}

// ValidateCheckNames returns an error if any name is unknown.
func ValidateCheckNames(names []string) error {
	for _, name := range names {
		if !Known(name) {
			return fmt.Errorf("unknown pedantic check %q (known: %s)", name, strings.Join(AllNames(), ", "))
		}
	}
	return nil
}
