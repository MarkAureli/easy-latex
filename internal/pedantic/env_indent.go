package pedantic

import (
	"fmt"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// envIndentWidth is the number of spaces per nesting level. Reuses the value
// applied by no-tabs so both checks agree on column accounting.
const envIndentWidth = 4

// noIndentBodyEnvs are environments whose body is preserved verbatim: lines
// inside them are never re-indented and never linted for indent. Depth of
// these envs is also not counted (children would be skipped anyway).
var noIndentBodyEnvs = map[string]bool{
	"verbatim":   true,
	"Verbatim":   true,
	"BVerbatim":  true,
	"LVerbatim":  true,
	"lstlisting": true,
	"minted":     true,
	"comment":    true,
	"alltt":      true,
}

// transparentEnvs do not contribute to indent depth: their body indents at the
// surrounding level. Used so the bulk of a document is not shifted right by 4.
var transparentEnvs = map[string]bool{
	"document": true,
}

func init() {
	Register(Check{
		Name:   "env-indent",
		Phase:  PhaseSource,
		Source: checkEnvIndent,
		Fix:    fixEnvIndent,
	})
}

type envEventKind int

const (
	evNone envEventKind = iota
	evBegin
	evEnd
)

type envEvent struct {
	kind envEventKind
	name string
}

// classifyLine inspects the first non-whitespace token of stripped and reports
// whether the line opens or closes an environment. \[ and \] count as
// begin/end of a synthetic env named "\\[".
func classifyLine(stripped string) envEvent {
	i := leadingWS(stripped)
	if i >= len(stripped) || stripped[i] != '\\' {
		return envEvent{}
	}
	if name, n := matchBeginEnd(stripped, i, "begin"); n > 0 {
		return envEvent{kind: evBegin, name: name}
	}
	if name, n := matchBeginEnd(stripped, i, "end"); n > 0 {
		return envEvent{kind: evEnd, name: name}
	}
	if i+1 < len(stripped) {
		switch stripped[i+1] {
		case '[':
			return envEvent{kind: evBegin, name: "\\["}
		case ']':
			return envEvent{kind: evEnd, name: "\\["}
		}
	}
	return envEvent{}
}

// contributesDepth reports whether an env name pushes the indent counter.
// Document and verbatim families are flat for indent purposes.
func contributesDepth(name string) bool {
	if transparentEnvs[name] || noIndentBodyEnvs[name] {
		return false
	}
	return true
}

func anyNoIndentAncestor(stack []string) bool {
	for _, n := range stack {
		if noIndentBodyEnvs[n] {
			return true
		}
	}
	return false
}

// nextDecision computes the expected indent (in spaces) for the line and
// updates depth/stack state. ancestorSkip is true when the line lies inside a
// no-indent-body env and should be left untouched.
func nextDecision(stripped string, depth *int, stack *[]string) (expected int, ancestorSkip bool) {
	ev := classifyLine(stripped)
	ancestorSkip = anyNoIndentAncestor(*stack)
	d := *depth
	if ev.kind == evEnd && contributesDepth(ev.name) {
		d = max(*depth-1, 0)
	}
	switch ev.kind {
	case evBegin:
		*stack = append(*stack, ev.name)
		if contributesDepth(ev.name) {
			*depth++
		}
	case evEnd:
		if len(*stack) > 0 {
			top := (*stack)[len(*stack)-1]
			*stack = (*stack)[:len(*stack)-1]
			if contributesDepth(top) {
				*depth = max(*depth-1, 0)
			}
		}
	}
	return d * envIndentWidth, ancestorSkip
}

func checkEnvIndent(path string, lines []string) []Diagnostic {
	var diags []Diagnostic
	var stack []string
	depth := 0
	for li, line := range lines {
		expected, skip := nextDecision(line, &depth, &stack)
		if skip {
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		leadEnd := leadingWS(line)
		actual := line[:leadEnd]
		if leadEnd == expected && actual == strings.Repeat(" ", expected) {
			continue
		}
		diags = append(diags, Diagnostic{
			File:    path,
			Line:    li + 1,
			Message: fmt.Sprintf("indent should be %d space(s), got %d", expected, leadEnd),
		})
	}
	return diags
}

func fixEnvIndent(path string, lines []string) ([]string, bool) {
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = texscan.StripComment(l)
	}
	out := make([]string, len(lines))
	var stack []string
	depth := 0
	changed := false
	for li, raw := range lines {
		expected, skip := nextDecision(stripped[li], &depth, &stack)
		if skip {
			out[li] = raw
			continue
		}
		if strings.TrimSpace(raw) == "" {
			out[li] = raw
			continue
		}
		leadEnd := leadingWS(raw)
		newLine := strings.Repeat(" ", expected) + raw[leadEnd:]
		if newLine != raw {
			changed = true
		}
		out[li] = newLine
	}
	return out, changed
}
