package bib

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
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

func loadCache(auxDir string) cache {
	data, err := os.ReadFile(filepath.Join(auxDir, "bib.json"))
	if err != nil {
		return make(cache)
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil {
		// Unreadable or old-format cache: start fresh; entries will be re-validated.
		return make(cache)
	}
	// Remove entries that lack a Type: these are old-format entries that predate
	// the Type field. They will be re-allocated on the next parsebib run.
	for k, v := range c {
		if v.Type == "" {
			delete(c, k)
		}
	}
	return c
}

func saveCache(auxDir string, c cache) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[bib] warning: could not marshal cache: %v\n", err)
		return
	}
	if err := os.WriteFile(filepath.Join(auxDir, "bib.json"), data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[bib] warning: could not write %s: %v\n", filepath.Join(auxDir, "bib.json"), err)
	}
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

// LoadCacheKeys returns all canonical citation keys stored in the bib cache.
func LoadCacheKeys(auxDir string) []string {
	c := loadCache(auxDir)
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
