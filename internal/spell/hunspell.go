package spell

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Miss describes one misspelled word reported by hunspell.
type Miss struct {
	Word        string
	Col         int      // 1-based column within the input line
	Suggestions []string // may be nil
}

// hunspellMissingWarned ensures the "hunspell not installed" warning prints
// at most once per process.
var hunspellMissingWarned sync.Once

// HunspellAvailable reports whether the `hunspell` binary is in PATH. The
// dictionary for a given lang is probed lazily by StartHunspell (banner read).
// On binary-missing it logs a one-time warning and returns false.
func HunspellAvailable(_ string, warn io.Writer) bool {
	if _, err := exec.LookPath("hunspell"); err != nil {
		hunspellMissingWarned.Do(func() {
			fmt.Fprintln(warn, "warning: spell-check skipped: hunspell binary not found in PATH (install via `brew install hunspell` or your distro's package manager)")
		})
		return false
	}
	return true
}

// Hunspell wraps a long-lived `hunspell -a` pipe-mode process.
type Hunspell struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

// StartHunspell launches `hunspell -a -d <lang> [-p <personalDict>]`. Use
// HunspellAvailable first to guard.
func StartHunspell(lang, personalDict string) (*Hunspell, error) {
	args := []string{"-a", "-d", lang}
	if personalDict != "" {
		args = append(args, "-p", personalDict)
	}
	cmd := exec.Command("hunspell", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	br := bufio.NewReader(stdout)
	// Discard banner line, e.g. "@(#) International Ispell ...".
	if _, err := br.ReadString('\n'); err != nil {
		return nil, fmt.Errorf("hunspell: read banner: %w", err)
	}
	return &Hunspell{cmd: cmd, stdin: stdin, stdout: br}, nil
}

// Close shuts the pipe-mode process down.
func (h *Hunspell) Close() error {
	_ = h.stdin.Close()
	return h.cmd.Wait()
}

// CheckLine sends one line of text to hunspell and returns misspellings.
// The leading `^` byte forces text mode (no `*`/`+`/`-`/`@` interpretation).
func (h *Hunspell) CheckLine(line string) ([]Miss, error) {
	// Hunspell pipe mode is line-oriented; embed newlines would split tokens.
	// Replace any newline with space defensively.
	line = strings.ReplaceAll(line, "\n", " ")
	if _, err := fmt.Fprintf(h.stdin, "^%s\n", line); err != nil {
		return nil, err
	}
	var misses []Miss
	for {
		s, err := h.stdout.ReadString('\n')
		if err != nil {
			return misses, err
		}
		s = strings.TrimRight(s, "\r\n")
		if s == "" {
			// Blank line terminates the response for this input line.
			return misses, nil
		}
		if m, ok := parseHunspellLine(s); ok {
			misses = append(misses, m)
		}
	}
}

// parseHunspellLine parses one hunspell pipe-mode response line.
// Formats of interest:
//
//	*                              (correct)
//	+ ROOT                         (correct, derived from ROOT)
//	-                              (compound)
//	& word N off: sug1, sug2, ...  (miss with suggestions)
//	# word off                     (miss without suggestions)
func parseHunspellLine(s string) (Miss, bool) {
	if s == "" {
		return Miss{}, false
	}
	switch s[0] {
	case '*', '+', '-':
		return Miss{}, false
	case '&':
		// "& word N off: sug, sug"
		body := strings.TrimPrefix(s, "& ")
		head, tail, _ := strings.Cut(body, ":")
		fields := strings.Fields(head)
		if len(fields) < 3 {
			return Miss{}, false
		}
		off, err := strconv.Atoi(fields[2])
		if err != nil {
			return Miss{}, false
		}
		var sugs []string
		if tail != "" {
			for p := range strings.SplitSeq(tail, ",") {
				if p = strings.TrimSpace(p); p != "" {
					sugs = append(sugs, p)
				}
			}
		}
		// Account for the leading `^` we prepended to the input line.
		col := max(off, 1)
		return Miss{Word: fields[0], Col: col, Suggestions: sugs}, true
	case '#':
		body := strings.TrimPrefix(s, "# ")
		fields := strings.Fields(body)
		if len(fields) < 2 {
			return Miss{}, false
		}
		off, err := strconv.Atoi(fields[1])
		if err != nil {
			return Miss{}, false
		}
		col := max(off, 1)
		return Miss{Word: fields[0], Col: col}, true
	}
	return Miss{}, false
}
