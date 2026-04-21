package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBibList_Empty(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".el"), 0755)
	chdir(t, dir)

	cmd := bibListCmd
	var buf strings.Builder
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunBibList_WithEntries(t *testing.T) {
	dir := t.TempDir()
	elDir := filepath.Join(dir, ".el")
	os.MkdirAll(elDir, 0755)

	cache := map[string]any{
		"Smith2024FooBar": map[string]any{
			"source": "crossref",
			"type":   "article",
			"fields": map[string]string{
				"title":  "Foo Bar Baz",
				"author": "Smith, John and Doe, Jane",
				"doi":    "10.1/test",
			},
		},
	}
	data, _ := json.MarshalIndent(cache, "", "  ")
	os.WriteFile(filepath.Join(elDir, "bib.json"), data, 0644)

	chdir(t, dir)

	cmd := bibListCmd
	var buf strings.Builder
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 50); got != "short" {
		t.Errorf("expected %q, got %q", "short", got)
	}
	long := strings.Repeat("a", 60)
	got := truncate(long, 50)
	if len(got) != 50 {
		t.Errorf("expected len 50, got %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected ... suffix")
	}
}

func TestTruncateAuthor(t *testing.T) {
	if got := truncateAuthor("Smith, John"); got != "Smith, John" {
		t.Errorf("single author: got %q", got)
	}
	if got := truncateAuthor("Smith, John and Doe, Jane"); got != "Smith, John et al." {
		t.Errorf("multiple authors: got %q", got)
	}
}
