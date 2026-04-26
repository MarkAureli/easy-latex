package pedantic

import (
	"fmt"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// tabWidth is the column width used to expand tab characters to spaces. Tabs
// advance the visual column to the next multiple of tabWidth.
const tabWidth = 4

func init() {
	Register(Check{
		Name:   "no-tabs",
		Phase:  PhaseSource,
		Source: checkNoTabs,
		Fix:    fixNoTabs,
	})
}

// checkNoTabs flags tab characters outside verbatim regions. Lines are
// comment-stripped by the runner; tabs inside comments are invisible here.
func checkNoTabs(path string, lines []string) []Diagnostic {
	mask := regionMask(lines)
	var diags []Diagnostic
	for i, line := range lines {
		col := firstTabOutsideVerbatim(line, mask[i])
		if col < 0 {
			continue
		}
		diags = append(diags, Diagnostic{
			File:    path,
			Line:    i + 1,
			Message: fmt.Sprintf("tab character at column %d", col+1),
		})
	}
	return diags
}

// fixNoTabs expands tabs to spaces using a column-aware tabstop of tabWidth.
// Tabs inside verbatim regions are left untouched; tabs in comments are
// rewritten.
func fixNoTabs(path string, lines []string) ([]string, bool) {
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = texscan.StripComment(l)
	}
	mask := regionMask(stripped)
	changed := false
	for i, line := range lines {
		body := stripped[i]
		comment := line[len(body):]
		newBody, newComment, did := expandTabs(body, comment, mask[i])
		if did {
			lines[i] = newBody + newComment
			changed = true
		}
	}
	return lines, changed
}

// firstTabOutsideVerbatim returns the byte index of the first tab in line that
// is not in a verbatim region, or -1 if none.
func firstTabOutsideVerbatim(line string, mask []regionKind) int {
	for i := 0; i < len(line); i++ {
		if line[i] != '\t' {
			continue
		}
		if i < len(mask) && mask[i] == regVerbatim {
			continue
		}
		return i
	}
	return -1
}

// expandTabs rewrites tabs as spaces on a single source line. mask classifies
// bytes of body; tabs in regVerbatim regions are preserved. The comment tail
// (if any) is also rewritten, with visual-column tracking continuing from the
// end of the body.
func expandTabs(body, comment string, mask []regionKind) (string, string, bool) {
	changed := false
	col := 0
	var bb strings.Builder
	bb.Grow(len(body))
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '\t' {
			n := tabWidth - (col % tabWidth)
			if i < len(mask) && mask[i] == regVerbatim {
				bb.WriteByte('\t')
				col += n
				continue
			}
			for range n {
				bb.WriteByte(' ')
			}
			col += n
			changed = true
			continue
		}
		bb.WriteByte(c)
		col++
	}
	var cb strings.Builder
	cb.Grow(len(comment))
	for i := 0; i < len(comment); i++ {
		c := comment[i]
		if c == '\t' {
			n := tabWidth - (col % tabWidth)
			for range n {
				cb.WriteByte(' ')
			}
			col += n
			changed = true
			continue
		}
		cb.WriteByte(c)
		col++
	}
	return bb.String(), cb.String(), changed
}
