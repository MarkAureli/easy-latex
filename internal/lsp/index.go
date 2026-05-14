package lsp

import (
	"github.com/MarkAureli/easy-latex/internal/bib"
)

// BuildItems returns one completionItem per unique cite key loaded from the
// global bib cache. Items are loaded once at server start; restart to pick up
// new entries. Kind=18 (Reference) prevents editors from showing "unknown".
func BuildItems() []completionItem {
	keys := bib.LoadCacheKeys()
	items := make([]completionItem, 0, len(keys))
	for _, key := range keys {
		items = append(items, completionItem{Label: key, Kind: 18})
	}
	return items
}
