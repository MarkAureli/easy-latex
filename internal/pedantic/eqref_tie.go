package pedantic

import (
	"fmt"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func init() {
	Register(Check{
		Name:   "eqref-tie",
		Phase:  PhaseSource,
		Source: checkEqrefTie,
		Fix:    fixEqrefTie,
	})
}

// eqrefTieOK are bytes after which \eqref needs no tie: the tie itself, or an
// opening/joining character that never carries a line-break space
// (`(\eqref`, `{\eqref`, `\eqref{a}--\eqref{b}`, `\eqref{a}/\eqref{b}`).
var eqrefTieOK = map[byte]bool{
	'~': true, '(': true, '[': true, '{': true, '-': true, '/': true,
}

// eqrefTieEdit is one violation: \eqref at pos, with the whitespace run
// [wsStart, pos) before it to be replaced by `~` (empty when \eqref is glued
// directly to the preceding character; includes a misplaced tie in `~ \eqref`).
type eqrefTieEdit struct {
	wsStart, pos int
}

// findEqrefTieEdits returns the \eqref occurrences in the comment-stripped
// line that are neither the line's first content nor preceded by a tie.
// Only text regions are inspected (math spacing ignores ties; verbatim is
// opaque).
func findEqrefTieEdits(line string, mask []regionKind) []eqrefTieEdit {
	var edits []eqrefTieEdit
	lead := leadingWS(line)
	for i := 0; ; {
		j := strings.Index(line[i:], `\eqref`)
		if j < 0 {
			return edits
		}
		pos := i + j
		i = pos + len(`\eqref`)
		if i < len(line) && isLetter(line[i]) {
			continue // longer macro name (\eqrefs, ...)
		}
		if pos < len(mask) && mask[pos] != regText {
			continue // ties are a text-mode concern; verbatim is opaque
		}
		if pos == lead {
			continue // first thing on the line
		}
		ws := pos
		for ws > lead && (line[ws-1] == ' ' || line[ws-1] == '\t') {
			ws--
		}
		if ws == pos && eqrefTieOK[line[pos-1]] {
			continue
		}
		if ws < pos && line[ws-1] == '~' {
			ws-- // `~ \eqref`: keep the tie, drop the stray space
		}
		edits = append(edits, eqrefTieEdit{wsStart: ws, pos: pos})
	}
}

func checkEqrefTie(_ *Context, path string, lines []string) []Diagnostic {
	mask := regionMask(lines)
	var diags []Diagnostic
	for li, line := range lines {
		for _, e := range findEqrefTieEdits(line, mask[li]) {
			diags = append(diags, Diagnostic{
				File:    path,
				Line:    li + 1,
				Message: fmt.Sprintf("\\eqref should be preceded by '~' (column %d)", e.pos+1),
			})
		}
	}
	return diags
}

// fixEqrefTie replaces the whitespace before each flagged \eqref with `~`
// (inserting one when \eqref is glued to the preceding character). Comment
// tails are preserved verbatim.
func fixEqrefTie(_ *Context, path string, lines []string) ([]string, bool) {
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = texscan.StripComment(l)
	}
	mask := regionMask(stripped)
	changed := false
	for li, body := range stripped {
		edits := findEqrefTieEdits(body, mask[li])
		if len(edits) == 0 {
			continue
		}
		var b strings.Builder
		prev := 0
		for _, e := range edits {
			b.WriteString(body[prev:e.wsStart])
			b.WriteByte('~')
			prev = e.pos
		}
		b.WriteString(body[prev:])
		lines[li] = b.String() + lines[li][len(body):]
		changed = true
	}
	return lines, changed
}
