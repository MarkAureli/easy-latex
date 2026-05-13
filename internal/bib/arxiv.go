package bib

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

type arxivFeed struct {
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	ID              string        `xml:"id"`
	Title           string        `xml:"title"`
	Authors         []arxivAuthor `xml:"author"`
	Published       string        `xml:"published"`
	DOI             string        `xml:"http://arxiv.org/schemas/atom doi"`
	PrimaryCategory struct {
		Term string `xml:"term,attr"`
	} `xml:"http://arxiv.org/schemas/atom primary_category"`
}

type arxivAuthor struct {
	Name string `xml:"name"`
}

// arxivBatchSize is the per-request id_list size used by PrefetchArxivIDs.
// arXiv's API documentation allows up to 2000, but smaller chunks keep latency
// and memory predictable.
const arxivBatchSize = 100

var (
	arxivPrefetchMu sync.Mutex
	arxivPrefetch   = map[string]*arxivEntry{}
	reArxivAtomID   = regexp.MustCompile(`v\d+$`)
)

// arxivCanonical returns the lowercase, version-less form of an arXiv id used
// as the prefetch cache key.
func arxivCanonical(id string) string {
	id = strings.TrimSpace(id)
	id = reArxivAtomID.ReplaceAllString(id, "")
	return strings.ToLower(id)
}

// extractArxivIDFromAtomID returns the bare arXiv id (no version, no URL prefix)
// from an Atom <id> like "http://arxiv.org/abs/2301.00001v1".
func extractArxivIDFromAtomID(s string) string {
	for _, prefix := range []string{"http://arxiv.org/abs/", "https://arxiv.org/abs/"} {
		if strings.HasPrefix(s, prefix) {
			s = s[len(prefix):]
			break
		}
	}
	return arxivCanonical(s)
}

// PrefetchArxivIDs fetches metadata for the given arXiv ids in chunked batches
// and populates an in-process cache consulted by queryArxiv. Failures are
// non-fatal: any id not satisfied here falls back to a per-id fetch.
func PrefetchArxivIDs(ids []string, log Logger) {
	log = logOrNop(log)
	if len(ids) == 0 {
		return
	}
	// Dedup, skip already-prefetched.
	seen := map[string]bool{}
	var todo []string
	arxivPrefetchMu.Lock()
	for _, id := range ids {
		key := arxivCanonical(id)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if _, ok := arxivPrefetch[key]; ok {
			continue
		}
		todo = append(todo, key)
	}
	arxivPrefetchMu.Unlock()
	if len(todo) == 0 {
		return
	}

	for start := 0; start < len(todo); start += arxivBatchSize {
		end := min(start+arxivBatchSize, len(todo))
		chunk := todo[start:end]
		log.Progress("", fmt.Sprintf("prefetching %d arXiv entries...", len(chunk)))
		fetchArxivBatch(chunk, log)
	}
}

func fetchArxivBatch(ids []string, log Logger) {
	apiURL := "https://export.arxiv.org/api/query?max_results=" +
		fmt.Sprintf("%d", len(ids)) +
		"&id_list=" + url.QueryEscape(strings.Join(ids, ","))
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "easy-latex/"+Version+" (https://github.com/MarkAureli/easy-latex)")
	resp, err := doWithRetry(func() (*http.Response, error) {
		return httpClient.Do(req)
	}, log, "")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var feed arxivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return
	}
	arxivPrefetchMu.Lock()
	defer arxivPrefetchMu.Unlock()
	for i := range feed.Entries {
		ax := &feed.Entries[i]
		key := extractArxivIDFromAtomID(ax.ID)
		if key == "" {
			continue
		}
		arxivPrefetch[key] = ax
	}
}

// lookupArxivPrefetch returns the cached entry for id (case-insensitive,
// version-stripped) and removes it from the cache so re-queries hit the network.
func lookupArxivPrefetch(id string) *arxivEntry {
	key := arxivCanonical(id)
	arxivPrefetchMu.Lock()
	defer arxivPrefetchMu.Unlock()
	ax, ok := arxivPrefetch[key]
	if !ok {
		return nil
	}
	delete(arxivPrefetch, key)
	return ax
}

func queryArxiv(e Entry, id string, log Logger) (*Entry, cacheEntry, string, error) {
	log = logOrNop(log)
	if ax := lookupArxivPrefetch(id); ax != nil {
		return processArxivEntry(e, id, ax)
	}
	log.Progress(e.Key, "fetching metadata from arXiv...")
	apiURL := "https://export.arxiv.org/api/query?id_list=" + url.QueryEscape(id)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, cacheEntry{}, "", err
	}
	req.Header.Set("User-Agent", "easy-latex/"+Version+" (https://github.com/MarkAureli/easy-latex)")
	resp, err := doWithRetry(func() (*http.Response, error) {
		return httpClient.Do(req)
	}, log, e.Key)
	if err != nil {
		if isRetryableError(err) {
			return nil, cacheEntry{}, "", fmt.Errorf("arXiv request timed out")
		}
		return nil, cacheEntry{}, "", fmt.Errorf("arXiv query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, cacheEntry{}, "", friendlyHTTPError(resp.StatusCode, "arXiv")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, cacheEntry{}, "", err
	}

	var feed arxivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, cacheEntry{}, "", err
	}
	if len(feed.Entries) == 0 {
		return nil, cacheEntry{}, "", fmt.Errorf("no entry found for %s: %w", id, errNotFound)
	}
	return processArxivEntry(e, id, &feed.Entries[0])
}

func processArxivEntry(e Entry, id string, ax *arxivEntry) (*Entry, cacheEntry, string, error) {
	// arXiv returns an error entry (no DOI, no authors, title "Error") for
	// malformed ids; treat as not-found so callers report a clean error.
	if strings.Contains(ax.ID, "arxiv.org/api/errors") {
		return nil, cacheEntry{}, "", fmt.Errorf("no entry found for %s: %w", id, errNotFound)
	}
	doi := strings.TrimSpace(ax.DOI)
	updated := e
	raw := cacheEntry{Fields: make(map[string]string)}
	var corrections []string

	title := strings.TrimSpace(strings.ReplaceAll(ax.Title, "\n", " "))
	if title != "" {
		raw.Fields["title"] = stripNonEscapedBraces(title)
		if applyField(&updated, "title", raw.Fields["title"]) {
			corrections = append(corrections, "title")
		}
	}
	if len(ax.Authors) > 0 {
		raw.Fields["author"] = formatArxivAuthors(ax.Authors)
		if applyField(&updated, "author", raw.Fields["author"]) {
			corrections = append(corrections, "author")
		}
	}
	if len(ax.Published) >= 4 {
		raw.Fields["year"] = ax.Published[:4]
		if applyField(&updated, "year", raw.Fields["year"]) {
			corrections = append(corrections, "year")
		}
	}
	raw.Fields["eprint"] = id
	if applyField(&updated, "eprint", id) {
		corrections = append(corrections, "eprint")
	}
	if pc := ax.PrimaryCategory.Term; pc != "" {
		raw.Fields["primaryclass"] = pc
		if applyField(&updated, "primaryclass", pc) {
			corrections = append(corrections, "primaryclass")
		}
	}

	if len(corrections) > 0 {
		return &updated, raw, doi, nil
	}
	return nil, raw, doi, nil
}

func formatArxivAuthors(authors []arxivAuthor) string {
	parts := make([]string, 0, len(authors))
	for _, a := range authors {
		name := normalizeAllCapsName(strings.TrimSpace(a.Name))
		if name != "" {
			parts = append(parts, reverseArxivName(name))
		}
	}
	return strings.Join(parts, " and ")
}

// reverseArxivName converts "First Last" → "Last, First".
// Names already in "Last, First" form are returned unchanged.
func reverseArxivName(name string) string {
	if strings.Contains(name, ",") {
		return name
	}
	parts := strings.Fields(name)
	if len(parts) < 2 {
		return name
	}
	return parts[len(parts)-1] + ", " + strings.Join(parts[:len(parts)-1], " ")
}
