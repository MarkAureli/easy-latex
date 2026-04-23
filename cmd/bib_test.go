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

func setupBibListDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	elDir := filepath.Join(dir, ".el")
	os.MkdirAll(elDir, 0755)

	cache := map[string]any{
		"Smith2024FooBar": map[string]any{
			"source": "crossref",
			"type":   "article",
			"fields": map[string]string{
				"title":  "Foo Bar Baz",
				"author": "Smith, John",
				"doi":    "10.1/test",
			},
		},
		"Jones2023Qux": map[string]any{
			"source": "no-id",
			"type":   "misc",
			"fields": map[string]string{
				"title":  "Qux Quux",
				"author": "Jones, Alice",
			},
		},
		"Doe2022Xyz": map[string]any{
			"source": "crossref",
			"type":   "book",
			"fields": map[string]string{
				"title":  "Xyz Book",
				"author": "Doe, Bob",
				"doi":    "10.2/test",
			},
		},
	}
	data, _ := json.MarshalIndent(cache, "", "  ")
	os.WriteFile(filepath.Join(elDir, "bib.json"), data, 0644)

	// Config pointing at main.tex
	cfg := map[string]any{"main": "main.tex"}
	cfgData, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(elDir, "config.json"), cfgData, 0644)

	// main.tex cites only Smith2024FooBar and Doe2022Xyz
	os.WriteFile(filepath.Join(dir, "main.tex"), []byte(`\begin{document}
\cite{Smith2024FooBar}
\cite{Doe2022Xyz}
\end{document}
`), 0644)

	return dir
}

func TestRunBibList_GroupedOutput(t *testing.T) {
	dir := setupBibListDir(t)
	chdir(t, dir)

	cmd := bibListCmd
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.Flags().Set("cited", "false")
	cmd.Flags().Set("uncited", "false")
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Referenced") {
		t.Error("expected Referenced section header")
	}
	if !strings.Contains(out, "Unreferenced") {
		t.Error("expected Unreferenced section header")
	}
	if !strings.Contains(out, "Smith2024FooBar") {
		t.Error("expected Smith2024FooBar in output")
	}
	if !strings.Contains(out, "Doe2022Xyz") {
		t.Error("expected Doe2022Xyz in output")
	}
	if !strings.Contains(out, "Jones2023Qux") {
		t.Error("expected Jones2023Qux in output")
	}
}

func TestRunBibList_CitedFlag(t *testing.T) {
	dir := setupBibListDir(t)
	chdir(t, dir)

	cmd := bibListCmd
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.Flags().Set("cited", "true")
	cmd.Flags().Set("uncited", "false")
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Smith2024FooBar") {
		t.Error("expected Smith2024FooBar in cited output")
	}
	if !strings.Contains(out, "Doe2022Xyz") {
		t.Error("expected Doe2022Xyz in cited output")
	}
	if strings.Contains(out, "Jones2023Qux") {
		t.Error("Jones2023Qux should not appear with --cited")
	}
}

func TestRunBibList_UncitedFlag(t *testing.T) {
	dir := setupBibListDir(t)
	chdir(t, dir)

	cmd := bibListCmd
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.Flags().Set("cited", "false")
	cmd.Flags().Set("uncited", "true")
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "Smith2024FooBar") {
		t.Error("Smith2024FooBar should not appear with --uncited")
	}
	if strings.Contains(out, "Doe2022Xyz") {
		t.Error("Doe2022Xyz should not appear with --uncited")
	}
	if !strings.Contains(out, "Jones2023Qux") {
		t.Error("expected Jones2023Qux in uncited output")
	}
}

func TestRunBibList_NoConfigFallback(t *testing.T) {
	// No config.json → no tex scanning → flat list, no grouping
	dir := t.TempDir()
	elDir := filepath.Join(dir, ".el")
	os.MkdirAll(elDir, 0755)

	cache := map[string]any{
		"Smith2024FooBar": map[string]any{
			"source": "crossref",
			"type":   "article",
			"fields": map[string]string{"title": "Foo"},
		},
	}
	data, _ := json.MarshalIndent(cache, "", "  ")
	os.WriteFile(filepath.Join(elDir, "bib.json"), data, 0644)

	chdir(t, dir)

	cmd := bibListCmd
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.Flags().Set("cited", "false")
	cmd.Flags().Set("uncited", "false")
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Smith2024FooBar") {
		t.Error("expected Smith2024FooBar in output")
	}
	// No grouping headers when config unavailable
	if strings.Contains(out, "Referenced") {
		t.Error("should not show Referenced header without config")
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
