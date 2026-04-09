package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// invokeConfig builds a fresh cobra.Command with all config flags,
// parses args, and calls runConfig. This avoids shared flag state between tests.
func invokeConfig(t *testing.T, args []string) error {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&flagAbbreviateJournals, "abbreviate-journals", true, "")
	cmd.Flags().BoolVar(&flagBraceTitles, "brace-titles", false, "")
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	return runConfig(cmd, nil)
}

func readAbbreviateJournals(t *testing.T, dir string) *bool {
	t.Helper()
	return readConfig(t, dir).AbbreviateJournals
}

func readBraceTitles(t *testing.T, dir string) *bool {
	t.Helper()
	return readConfig(t, dir).BraceTitles
}

// ── abbreviateJournals() helper ────────────────────────────────────────────────

func TestAbbreviateJournals_NilDefaultsTrue(t *testing.T) {
	cfg := &Config{}
	if !cfg.abbreviateJournals() {
		t.Error("expected true when AbbreviateJournals is nil")
	}
}

func TestAbbreviateJournals_ExplicitTrue(t *testing.T) {
	v := true
	cfg := &Config{AbbreviateJournals: &v}
	if !cfg.abbreviateJournals() {
		t.Error("expected true when AbbreviateJournals is &true")
	}
}

func TestAbbreviateJournals_ExplicitFalse(t *testing.T) {
	v := false
	cfg := &Config{AbbreviateJournals: &v}
	if cfg.abbreviateJournals() {
		t.Error("expected false when AbbreviateJournals is &false")
	}
}

// ── el config command ─────────────────────────────────────────────────────────

func TestRunConfig_NotInitialized(t *testing.T) {
	chdir(t, t.TempDir())
	if err := invokeConfig(t, []string{"--abbreviate-journals=false"}); err == nil {
		t.Fatal("expected error when .el.json missing, got nil")
	}
}

func TestRunConfig_NoFlags(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfig(t, nil); err == nil {
		t.Fatal("expected error when no flags passed, got nil")
	}
}

func TestRunConfig_SetFalse(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfig(t, []string{"--abbreviate-journals=false"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readAbbreviateJournals(t, dir)
	if got == nil || *got != false {
		t.Errorf("AbbreviateJournals = %v, want &false", got)
	}
}

func TestRunConfig_SetTrue(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfig(t, []string{"--abbreviate-journals=true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readAbbreviateJournals(t, dir)
	if got == nil || *got != true {
		t.Errorf("AbbreviateJournals = %v, want &true", got)
	}
}

func TestRunConfig_PreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	// Write a config with bib files set
	cfg := Config{Main: "thesis.tex", AuxDir: ".aux_dir", BibFiles: []string{"refs.bib", "extra.bib"}}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(dir, ".el.json"), data, 0644)
	chdir(t, dir)

	if err := invokeConfig(t, []string{"--abbreviate-journals=false"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := readConfig(t, dir)
	if updated.Main != "thesis.tex" {
		t.Errorf("Main = %q, want %q", updated.Main, "thesis.tex")
	}
	if updated.AuxDir != ".aux_dir" {
		t.Errorf("AuxDir = %q, want %q", updated.AuxDir, ".aux_dir")
	}
	if len(updated.BibFiles) != 2 {
		t.Errorf("BibFiles = %v, want 2 entries", updated.BibFiles)
	}
}

func TestRunConfig_ToggleValue(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	// Disable abbreviation
	if err := invokeConfig(t, []string{"--abbreviate-journals=false"}); err != nil {
		t.Fatalf("disable: unexpected error: %v", err)
	}
	got := readAbbreviateJournals(t, dir)
	if got == nil || *got != false {
		t.Errorf("after disable: AbbreviateJournals = %v, want &false", got)
	}

	// Re-enable abbreviation
	if err := invokeConfig(t, []string{"--abbreviate-journals=true"}); err != nil {
		t.Fatalf("enable: unexpected error: %v", err)
	}
	got = readAbbreviateJournals(t, dir)
	if got == nil || *got != true {
		t.Errorf("after enable: AbbreviateJournals = %v, want &true", got)
	}
}

// ── braceTitles() helper ──────────────────────────────────────────────────────

func TestBraceTitles_NilDefaultsFalse(t *testing.T) {
	cfg := &Config{}
	if cfg.braceTitles() {
		t.Error("expected false when BraceTitles is nil")
	}
}

func TestBraceTitles_ExplicitTrue(t *testing.T) {
	v := true
	cfg := &Config{BraceTitles: &v}
	if !cfg.braceTitles() {
		t.Error("expected true when BraceTitles is &true")
	}
}

func TestBraceTitles_ExplicitFalse(t *testing.T) {
	v := false
	cfg := &Config{BraceTitles: &v}
	if cfg.braceTitles() {
		t.Error("expected false when BraceTitles is &false")
	}
}

// ── --brace-titles flag ───────────────────────────────────────────────────────

func TestRunConfig_SetBraceTitlesTrue(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfig(t, []string{"--brace-titles=true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readBraceTitles(t, dir)
	if got == nil || *got != true {
		t.Errorf("BraceTitles = %v, want &true", got)
	}
}

func TestRunConfig_SetBraceTitlesFalse(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfig(t, []string{"--brace-titles=false"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readBraceTitles(t, dir)
	if got == nil || *got != false {
		t.Errorf("BraceTitles = %v, want &false", got)
	}
}

func TestRunConfig_BothFlagsAtOnce(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfig(t, []string{"--abbreviate-journals=false", "--brace-titles=true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := readConfig(t, dir)
	if cfg.AbbreviateJournals == nil || *cfg.AbbreviateJournals != false {
		t.Errorf("AbbreviateJournals = %v, want &false", cfg.AbbreviateJournals)
	}
	if cfg.BraceTitles == nil || *cfg.BraceTitles != true {
		t.Errorf("BraceTitles = %v, want &true", cfg.BraceTitles)
	}
}

func TestRunConfig_BraceTitlesOmittedFromFreshConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	cfg := readConfig(t, dir)
	if cfg.BraceTitles != nil {
		t.Errorf("fresh config: BraceTitles = %v, want nil", cfg.BraceTitles)
	}
	if cfg.braceTitles() {
		t.Error("fresh config: braceTitles() = true, want false")
	}
}

func TestRunConfig_OmittedFromFreshConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	// A fresh config written by writeConfig should have nil AbbreviateJournals
	// (omitted from JSON), which defaults to true.
	cfg := readConfig(t, dir)
	if cfg.AbbreviateJournals != nil {
		t.Errorf("fresh config: AbbreviateJournals = %v, want nil", cfg.AbbreviateJournals)
	}
	if !cfg.abbreviateJournals() {
		t.Error("fresh config: abbreviateJournals() = false, want true")
	}
}
