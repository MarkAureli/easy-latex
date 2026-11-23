package pedantic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
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
	PhaseSource        Phase = iota // runs per-file on tex source
	PhaseProjectSource              // runs once with all tex source files at hand
	PhasePostCompile                // runs after final pdflatex pass
)

// Context carries per-project data shared by source checks. A nil *Context is
// valid and behaves like an empty one.
type Context struct {
	// CmdArgs maps command names (without backslash; starred names separate)
	// to their number of mandatory arguments. Commands absent from it are
	// unknown. See texscan.ProjectCmdArgs / texscan.StoredCmdArgs.
	CmdArgs map[string]int
}

// Args returns the number of mandatory arguments of command name and
// whether it is known.
func (c *Context) Args(name string) (int, bool) {
	if c == nil {
		return 0, false
	}
	n, ok := c.CmdArgs[name]
	return n, ok
}

// SourceCheckFunc checks source lines (comment-stripped) for a single file.
type SourceCheckFunc func(ctx *Context, path string, lines []string) []Diagnostic

// SourceFixFunc rewrites raw source lines (NOT comment-stripped) for a single
// file. Returns the new lines and true when modifications were made, or the
// input slice and false when no change is needed. Implementations are
// responsible for being comment-aware where relevant.
type SourceFixFunc func(ctx *Context, path string, lines []string) ([]string, bool)

// ProjectSourceCheckFunc inspects every tex file in the project at once.
// files maps path → comment-stripped lines. Read-only; no autofix.
type ProjectSourceCheckFunc func(files map[string][]string) []Diagnostic

// PostCompileCheckFunc runs after all pdflatex passes complete.
// auxDir is the .el/ directory containing build artifacts.
type PostCompileCheckFunc func(auxDir string) []Diagnostic

// Check describes a registered pedantic check.
//
// Source-phase checks may optionally provide Fix to enable autofix; pure
// linters leave Fix nil. Project-source and post-compile checks are read-only.
//
// WantRaw (source phase only): when true, Source receives raw lines (NOT
// comment-stripped). Default is false; runner passes comment-stripped lines.
type Check struct {
	Name          string
	Phase         Phase
	Source        SourceCheckFunc
	Fix           SourceFixFunc
	WantRaw       bool
	ProjectSource ProjectSourceCheckFunc
	PostCompile   PostCompileCheckFunc
	// StyName + Sty: for PhasePostCompile checks that need a LaTeX package
	// auto-injected into the build. StyName is the filename (e.g.
	// "el-mathpos.sty"); Sty is its embedded bytes. The package basename
	// (without .sty) is \RequirePackage'd before the document.
	StyName string
	Sty     []byte
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

// stripComments returns the comment-stripped body of every line.
func stripComments(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = texscan.StripComment(l)
	}
	return out
}
