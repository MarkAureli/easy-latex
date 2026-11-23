package pedantic

import (
	"fmt"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

func init() {
	Register(Check{
		Name:   "cite-tie",
		Phase:  PhaseSource,
		Source: checkCiteTie,
		Fix:    fixCiteTie,
	})
}

// citeTieEdit is one violation: a cite command at pos, with the whitespace
// run [wsStart, pos) before it to be replaced by `~` (empty when the command
// is glued directly to the preceding character; includes a misplaced tie in
// `~ \cite{a}`).
type citeTieEdit struct {
	wsStart, pos int
}

// findCiteTieEdits returns the cite-command occurrences (texscan.ReCiteCall:
// \cite, \citep, \citet, \citeauthor, \parencite, \textcite, \autocite,
// \fullcite, \citealt, \citealp, capitalized and starred variants — the same
// set no-block-citations scans) in the comment-stripped line that are
// neither the line's first content nor preceded by a tie. Only text regions
// are inspected (math spacing ignores ties; verbatim is opaque).
func findCiteTieEdits(line string, mask []regionKind) []citeTieEdit {
	var edits []citeTieEdit
	lead := leadingWS(line)
	for _, m := range texscan.ReCiteCall.FindAllStringIndex(line, -1) {
		pos := m[0]
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
			ws-- // `~ \cite{a}`: keep the tie, drop the stray space
		}
		edits = append(edits, citeTieEdit{wsStart: ws, pos: pos})
	}
	return edits
}

func checkCiteTie(_ *Context, path string, lines []string) []Diagnostic {
	mask := regionMask(lines)
	var diags []Diagnostic
	for li, line := range lines {
		for _, e := range findCiteTieEdits(line, mask[li]) {
			diags = append(diags, Diagnostic{
				File:    path,
				Line:    li + 1,
				Message: fmt.Sprintf("citation should be preceded by '~' (column %d)", e.pos+1),
			})
		}
	}
	return diags
}

// fixCiteTie replaces the whitespace before each flagged cite command with
// `~` (inserting one when the command is glued to the preceding character).
// Comment tails are preserved verbatim.
func fixCiteTie(_ *Context, path string, lines []string) ([]string, bool) {
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = texscan.StripComment(l)
	}
	mask := regionMask(stripped)
	changed := false
	for li, body := range stripped {
		edits := findCiteTieEdits(body, mask[li])
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
