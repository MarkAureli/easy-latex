package bib

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// cacheEntry stores the validation source and a snapshot of all allowed field
// values for an entry. Fields holds pre-config-transform values so that changes
// to abbreviateJournals, maxAuthors, abbreviateFirstName, braceTitles, or
// urlFromDOI take effect on the next compile without re-fetching from the API.
//
// API-sourced fields stored raw:
//   - title: after stripNonEscapedBraces (non-configurable), before brace-wrapping
//   - author: full "Last, First and …" string, before maxAuthors/abbreviation
//   - journal: full unabbreviated name (Crossref only)
//   - url: as present in the bib file before urlFromDOI; absent when not in bib
//
// All other allowed fields (year, volume, number, pages, doi, booktitle, …) are
// stored as returned by the API or as present in the bib file.
type cacheEntry struct {
	Source string            `json:"source"`
	Type   string            `json:"type"`
	Fields map[string]string `json:"fields,omitempty"`
}

// cache maps canonical citation keys to their cacheEntry.
type cache map[string]cacheEntry

// errCorruptCache is returned by loadCacheStrict when the global bib cache
// exists but cannot be parsed as valid JSON.
var errCorruptCache = fmt.Errorf("global bib cache exists but contains invalid JSON; please fix or delete it")

func loadCache() cache {
	c, _ := loadCacheStrict()
	return c
}

func loadCacheStrict() (cache, error) {
	path, err := GlobalBibPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(cache), nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return make(cache), nil
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, errCorruptCache
	}
	// Remove entries that lack a Type: these are old-format entries that predate
	// the Type field. They will be re-allocated on the next bib parse run.
	for k, v := range c {
		if v.Type == "" {
			delete(c, k)
		}
	}
	return c, nil
}

// saveCache writes the cache to the global bib path atomically via tmp+rename.
// Callers are expected to hold the global lock via withGlobalLock when doing a
// read-modify-write sequence.
func saveCache(c cache, log Logger) {
	log = logOrNop(log)
	path, err := ensureGlobalDir()
	if err != nil {
		log.Warn("", err.Error())
		return
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Warn("", fmt.Sprintf("could not marshal cache: %v", err))
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		log.Warn("", fmt.Sprintf("could not write %s: %v", tmp, err))
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Warn("", fmt.Sprintf("could not rename %s -> %s: %v", tmp, path, err))
		_ = os.Remove(tmp)
	}
}

// withGlobalLock serializes read-modify-write access to the global bib cache
// across processes via an advisory flock on a sibling lock file. The lock file
// path is stable (never renamed) so it remains a reliable mutex even when the
// cache file is replaced atomically.
func withGlobalLock(fn func() error) error {
	path, err := ensureGlobalDir()
	if err != nil {
		return err
	}
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("could not open %s: %w", lockPath, err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("could not lock %s: %w", lockPath, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return fn()
}

// LoadRenames reads .el/renames.json and returns the old→new key map.
// Returns an empty map if the file is absent or unreadable.
func LoadRenames(auxDir string) map[string]string {
	data, err := os.ReadFile(filepath.Join(auxDir, "renames.json"))
	if err != nil {
		return map[string]string{}
	}
	var r map[string]string
	if err := json.Unmarshal(data, &r); err != nil {
		return map[string]string{}
	}
	return r
}

// SaveRenames merges renames into .el/renames.json (creates if absent).
func SaveRenames(auxDir string, renames map[string]string) {
	if len(renames) == 0 {
		return
	}
	existing := LoadRenames(auxDir)
	maps.Copy(existing, renames)
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[bib] warning: could not marshal renames: %v\n", err)
		return
	}
	if err := os.WriteFile(filepath.Join(auxDir, "renames.json"), data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[bib] warning: could not write %s: %v\n", filepath.Join(auxDir, "renames.json"), err)
	}
}

// ClearRenames removes .el/renames.json.
func ClearRenames(auxDir string) {
	_ = os.Remove(filepath.Join(auxDir, "renames.json"))
}

// BibFileChanged reports whether bibPath has changed since the last
// UpdateBibHash call. Returns false if the file cannot be read.
func BibFileChanged(bibPath, auxDir string) bool {
	data, err := os.ReadFile(bibPath)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum) != loadBibHash(auxDir)
}

// UpdateBibHash computes the SHA-256 hash of bibPath and saves it to
// auxDir/bib_hash.
func UpdateBibHash(bibPath, auxDir string) {
	data, err := os.ReadFile(bibPath)
	if err != nil {
		return
	}
	sum := sha256.Sum256(data)
	if err := os.WriteFile(filepath.Join(auxDir, "bib_hash"), fmt.Appendf(nil, "%x", sum), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[bib] warning: could not write %s: %v\n", filepath.Join(auxDir, "bib_hash"), err)
	}
}

func loadBibHash(auxDir string) string {
	data, err := os.ReadFile(filepath.Join(auxDir, "bib_hash"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// LoadCacheKeys returns all canonical citation keys stored in the global bib
// cache.
func LoadCacheKeys() []string {
	c := loadCache()
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// RemoveEntryFromCache deletes the entry with the given key from the global
// bib cache. Returns true if the key existed and was removed, false if not
// found.
func RemoveEntryFromCache(key string) (bool, error) {
	removed, _, err := RemoveEntriesFromCache([]string{key})
	if err != nil {
		return false, err
	}
	return len(removed) == 1, nil
}

// RemoveEntriesFromCache deletes multiple entries from the global bib cache in
// a single locked read-modify-write cycle. Returns the keys actually removed
// and the keys that were not found in the cache.
func RemoveEntriesFromCache(keys []string) (removed, notFound []string, err error) {
	err = withGlobalLock(func() error {
		c, err := loadCacheStrict()
		if err != nil {
			return err
		}
		for _, k := range keys {
			if _, ok := c[k]; ok {
				delete(c, k)
				removed = append(removed, k)
			} else {
				notFound = append(notFound, k)
			}
		}
		if len(removed) > 0 {
			saveCache(c, nil)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return removed, notFound, nil
}

// CacheEntryInfo holds summary information about a cached bib entry.
type CacheEntryInfo struct {
	Key    string
	Type   string
	Source string
	Title  string
	Author string
}

// LoadCacheEntries returns summary information for all entries in the global
// bib cache, sorted by key.
func LoadCacheEntries() []CacheEntryInfo {
	c := loadCache()
	entries := make([]CacheEntryInfo, 0, len(c))
	for key, e := range c {
		entries = append(entries, CacheEntryInfo{
			Key:    key,
			Type:   e.Type,
			Source: e.Source,
			Title:  e.Fields["title"],
			Author: e.Fields["author"],
		})
	}
	slices.SortFunc(entries, func(a, b CacheEntryInfo) int {
		return strings.Compare(a.Key, b.Key)
	})
	return entries
}
