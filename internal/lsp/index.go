package lsp

import (
	"os"

	"github.com/MarkAureli/easy-latex/internal/bib"
)

// BuildItems parses bibFiles and returns one completionItem per unique cite key.
// Items are loaded once at server start; restart to pick up new entries.
func BuildItems(bibFiles []string) []completionItem {
	var items []completionItem
	seen := make(map[string]bool)
	for _, path := range bibFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, item := range bib.ParseFile(string(data)) {
			if !item.IsEntry {
				continue
			}
			e := item.Entry
			if seen[e.Key] {
				continue
			}
			seen[e.Key] = true
			items = append(items, completionItem{
				Label: e.Key,
			})
		}
	}
	return items
}
