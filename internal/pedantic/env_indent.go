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
	pos  int // byte offset of the token's `\` in the (comment-stripped) line
	end  int // byte offset one past the token (after the closing `}` for begin/end)
	kind envEventKind
	name string
}

// leadingContentPos returns the offset of the first non-whitespace, non-`{`
// character on a (comment-stripped) line. Skipping past leading `{` matches
// block-on-newline's carve-out for brace-wrapped bodies like
// `{\end{subequations}}` used as the second argument of \newenvironment.
func leadingContentPos(stripped string) int {
	i := leadingWS(stripped)
	for i < len(stripped) && stripped[i] == '{' {
		i++
		for i < len(stripped) && (stripped[i] == ' ' || stripped[i] == '\t') {
			i++
		}
	}
	return i
}

// scanLineEvents returns every `\begin{NAME}`, `\end{NAME}`, `\[`, and `\]`
// occurrence on the (comment-stripped) line, in source order. Synthetic env
// name `"\\["` represents display-math brackets. `\\` is consumed as a single
// token so embedded backslashes don't split into spurious matches.
func scanLineEvents(stripped string) []envEvent {
	var events []envEvent
	for i := 0; i < len(stripped); {
		if stripped[i] != '\\' {
			i++
			continue
		}
		if i+1 < len(stripped) && stripped[i+1] == '\\' {
			i += 2
			continue
		}
		if name, n := matchBeginEnd(stripped, i, "begin"); n > 0 {
			events = append(events, envEvent{pos: i, end: i + n, kind: evBegin, name: name})
			i += n
			continue
		}
		if name, n := matchBeginEnd(stripped, i, "end"); n > 0 {
			events = append(events, envEvent{pos: i, end: i + n, kind: evEnd, name: name})
			i += n
			continue
		}
		if i+1 < len(stripped) {
			switch stripped[i+1] {
			case '[':
				events = append(events, envEvent{pos: i, end: i + 2, kind: evBegin, name: "\\["})
				i += 2
				continue
			case ']':
				events = append(events, envEvent{pos: i, end: i + 2, kind: evEnd, name: "\\["})
				i += 2
				continue
			}
		}
		i++
	}
	return events
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

// braceCounts summarizes unmatched bracket activity on a single (comment-
// stripped) line. leadingCloses is the count of consecutive `}` or `]` at the
// start of the line (after whitespace); it determines how much the line itself
// de-dents relative to the previous line. opens and closes are totals across
// the whole line; their difference advances the running brace depth for
// subsequent lines.
type braceCounts struct {
	leadingCloses int
	opens         int
	closes        int
}

// countBraces tallies unescaped `{`/`[`/`}`/`]` on a line. A bracket is
// considered escaped when it is the character immediately following an
// unescaped `\`. This naturally skips `\{`, `\}`, `\[`, `\]`, and bracket-like
// characters appearing as the first letter of a control word (which never
// happens for letters anyway, but we treat `\` uniformly). Comments must be
// stripped by the caller.
func countBraces(stripped string) braceCounts {
	bc := braceCounts{}
	i := leadingWS(stripped)
	for i < len(stripped) {
		c := stripped[i]
		if c != '}' && c != ']' {
			break
		}
		bc.leadingCloses++
		i++
	}
	inEscape := false
	for j := 0; j < len(stripped); j++ {
		c := stripped[j]
		if inEscape {
			inEscape = false
			continue
		}
		if c == '\\' {
			inEscape = true
			continue
		}
		switch c {
		case '{', '[':
			bc.opens++
		case '}', ']':
			bc.closes++
		}
	}
	return bc
}

// nextDecision computes the expected indent (in spaces) for the line and
// updates depth/stack state. ancestorSkip is true when the line lies inside a
// no-indent-body env and should be left untouched. All begin/end events on the
// line update the env stack in order; the line's own depth is de-dented only
// when its first event sits at the line's leading-content position and is an
// end event (e.g. `\end{cases}` at line start, or `{\end{env}}`).
func nextDecision(line string, depth *int, stack *[]string, braceDepth *int) (expected int, ancestorSkip bool) {
	stripped := texscan.StripComment(line)
	events := scanLineEvents(stripped)
	ancestorSkip = anyNoIndentAncestor(*stack)
	leadPos := leadingContentPos(stripped)
	leadingEnds := 0
	cursor := leadPos
	for _, ev := range events {
		if ev.kind != evEnd || ev.pos != cursor || !contributesDepth(ev.name) {
			break
		}
		leadingEnds++
		cursor = ev.end
		for cursor < len(stripped) && (stripped[cursor] == ' ' || stripped[cursor] == '\t') {
			cursor++
		}
	}
	d := max(*depth-leadingEnds, 0)
	bc := countBraces(stripped)
	bd := max(*braceDepth-bc.leadingCloses, 0)
	for _, ev := range events {
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
	}
	if !ancestorSkip {
		*braceDepth = max(*braceDepth+bc.opens-bc.closes, 0)
	}
	return (d + bd) * envIndentWidth, ancestorSkip
}

func checkEnvIndent(path string, lines []string) []Diagnostic {
	var diags []Diagnostic
	var stack []string
	depth := 0
	braceDepth := 0
	for li, line := range lines {
		expected, skip := nextDecision(line, &depth, &stack, &braceDepth)
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
	braceDepth := 0
	changed := false
	for li, raw := range lines {
		expected, skip := nextDecision(stripped[li], &depth, &stack, &braceDepth)
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
