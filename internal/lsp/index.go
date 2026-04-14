package lsp

import (
	"github.com/MarkAureli/easy-latex/internal/bib"
)

// BuildItems returns one completionItem per unique cite key loaded from the bib cache.
// Items are loaded once at server start; restart to pick up new entries.
func BuildItems(auxDir string) []completionItem {
	keys := bib.LoadCacheKeys(auxDir)
	items := make([]completionItem, 0, len(keys))
	for _, key := range keys {
		items = append(items, completionItem{Label: key})
	}
	return items
}
