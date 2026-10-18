package lsp

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// reRefTrigger matches a reference command open brace with any content up to
// (but not including) the closing brace. Group 1 is the command name.
var reRefTrigger = regexp.MustCompile(`\\(ref|eqref|autoref|[cC]ref|[Vv]ref|pageref|nameref|labelcref|[cC]pageref)\*?(?:\[[^\]]*\])*\{[^}]*$`)

var reLabel = regexp.MustCompile(`\\label\{([^}]*)\}`)

// refPrefixFilter maps a reference command to the label prefix it requires.
var refPrefixFilter = map[string]string{
	"eqref": "equation:",
}

// detectRefPrefix returns the command name, the partially-typed label and true
// when the cursor is inside a \ref-like argument.
func detectRefPrefix(line string, char int) (cmd, prefix string, ok bool) {
	if char > len(line) {
		char = len(line)
	}
	sub := line[:char]
	m := reRefTrigger.FindStringSubmatch(sub)
	if m == nil {
		return "", "", false
	}
	idx := strings.LastIndexAny(sub, "{,")
	return m[1], strings.TrimSpace(sub[idx+1:]), true
}

// labelsIn returns all \label{...} keys in text, comments stripped.
func labelsIn(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		for _, m := range reLabel.FindAllStringSubmatch(texscan.StripComment(line), -1) {
			out = append(out, strings.TrimSpace(m[1]))
		}
	}
	return out
}

// docLabels collects labels from the in-memory buffer at uri plus every file
// reachable from it via \input/\include (read from disk). Sorted, deduped.
func (s *server) docLabels(uri string) []string {
	seen := map[string]bool{}
	var labels []string
	add := func(text string) {
		for _, l := range labelsIn(text) {
			if l != "" && !seen[l] {
				seen[l] = true
				labels = append(labels, l)
			}
		}
	}
	add(s.docs[uri])
	if path := uriToPath(uri); filepath.IsAbs(path) {
		for _, f := range texscan.FindTexFiles(filepath.Base(path), filepath.Dir(path)) {
			if f == path {
				continue
			}
			if open, ok := s.docs["file://"+f]; ok {
				add(open)
			} else if data, err := os.ReadFile(f); err == nil {
				add(string(data))
			}
		}
	}
	sort.Strings(labels)
	return labels
}

// completeRef returns label completions for a \ref-like command.
func (s *server) completeRef(uri, cmd, prefix string) []completionItem {
	need := refPrefixFilter[cmd]
	out := []completionItem{}
	for _, l := range s.docLabels(uri) {
		if strings.HasPrefix(l, need) && strings.HasPrefix(l, prefix) {
			out = append(out, completionItem{Label: l, Kind: 18})
		}
	}
	return out
}
