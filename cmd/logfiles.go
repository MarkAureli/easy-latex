package cmd

import (
	"regexp"
	"strings"
)

// reLogFileOpen matches a TeX log file-open marker: "(" followed by a path
// that starts with ./ or / or ends in .tex. File names may contain spaces and
// end at "[" (page marker), "(", ")" or end of line.
var reLogFileOpen = regexp.MustCompile(`^\(((?:\.{0,2}/)[^()\[\]]*?|[^()\[\]]*?\.tex)(?:\s*(?:[\[()]|$))`)

// logFileTracker follows the "(file ... )" nesting of TeX output so warnings
// can be attributed to the source file that was being read.
type logFileTracker struct {
	stack    []string // one entry per open "(" ; "" for non-file parens
	inBlock  bool     // inside a diagnostic block (until blank line)
	afterCtx bool     // previous line was an "l.N" context line
}

// current returns the innermost open file, or "".
func (t *logFileTracker) current() string {
	for i := len(t.stack) - 1; i >= 0; i-- {
		if t.stack[i] != "" {
			return t.stack[i]
		}
	}
	return ""
}

// feed processes one output line. isDiag marks lines that start a diagnostic
// block, whose content (font dumps, "(pkg)" continuations) is not scanned.
func (t *logFileTracker) feed(line string, isDiag bool) {
	if strings.TrimSpace(line) == "" {
		t.inBlock = false
		return
	}
	if t.afterCtx {
		// The line after "l.N ..." is the rest of the source line, and no blank
		// line follows it: the block ends here. It may carry page markers and
		// file closers ("      [1]) (./next.tex"), so scan it unless it looks
		// like plain source text.
		t.afterCtx = false
		if tr := strings.TrimSpace(line); tr != "" && strings.ContainsRune("[])", rune(tr[0])) {
			t.inBlock = false
		}
	}
	if contextLinePattern.MatchString(line) {
		t.afterCtx = true
		return
	}
	if isDiag {
		t.inBlock = true
	}
	if t.inBlock {
		return
	}
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '(':
			name := ""
			if m := reLogFileOpen.FindStringSubmatch(line[i:]); m != nil {
				name = strings.TrimPrefix(strings.TrimSpace(m[1]), "./")
				// Absolute paths are TeX distribution files (packages, classes):
				// keep attributing warnings to the project file that loaded them.
				if strings.HasPrefix(name, "/") {
					name = ""
				}
			}
			t.stack = append(t.stack, name)
		case ')':
			if len(t.stack) > 0 {
				t.stack = t.stack[:len(t.stack)-1]
			}
		}
	}
}
