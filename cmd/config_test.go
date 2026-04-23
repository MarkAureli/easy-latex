package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ── Test helpers ─────────────────────────────────────────────────────────────

// invokeConfigSet builds a fresh cobra.Command, parses args, and calls runConfigSet.
func invokeConfigSet(t *testing.T, args []string) error {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().Bool("global", false, "")
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	return runConfigSet(cmd, cmd.Flags().Args())
}

// invokeConfigUnset builds a fresh cobra.Command, parses args, and calls runConfigUnset.
func invokeConfigUnset(t *testing.T, args []string) error {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().Bool("global", false, "")
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	return runConfigUnset(cmd, cmd.Flags().Args())
}

func setGlobalConfigDir(t *testing.T, dir string) {
	t.Helper()
	orig := globalConfigDir
	globalConfigDir = dir
	t.Cleanup(func() { globalConfigDir = orig })
}

func readGlobalConfig(t *testing.T, dir string) Config {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".elconfig.json"))
	if err != nil {
		t.Fatalf("readGlobalConfig: %v", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("readGlobalConfig unmarshal: %v", err)
	}
	return cfg
}

// ── Config accessor tests (unchanged from before) ───────────────────────────

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

func TestIEEEFormat_NilDefaultsFalse(t *testing.T) {
	cfg := &Config{}
	if cfg.ieeeFormat() {
		t.Error("expected false when IEEEFormat is nil")
	}
}

func TestIEEEFormat_ExplicitTrue(t *testing.T) {
	v := true
	cfg := &Config{IEEEFormat: &v}
	if !cfg.ieeeFormat() {
		t.Error("expected true when IEEEFormat is &true")
	}
}

func TestIEEEFormat_ExplicitFalse(t *testing.T) {
	v := false
	cfg := &Config{IEEEFormat: &v}
	if cfg.ieeeFormat() {
		t.Error("expected false when IEEEFormat is &false")
	}
}

func TestAbbreviateFirstName_NilDefaultsTrue(t *testing.T) {
	cfg := &Config{}
	if !cfg.abbreviateFirstName() {
		t.Error("expected true when AbbreviateFirstName is nil")
	}
}

func TestAbbreviateFirstName_ExplicitTrue(t *testing.T) {
	v := true
	cfg := &Config{AbbreviateFirstName: &v}
	if !cfg.abbreviateFirstName() {
		t.Error("expected true when AbbreviateFirstName is &true")
	}
}

func TestAbbreviateFirstName_ExplicitFalse(t *testing.T) {
	v := false
	cfg := &Config{AbbreviateFirstName: &v}
	if cfg.abbreviateFirstName() {
		t.Error("expected false when AbbreviateFirstName is &false")
	}
}

func TestUrlFromDOI_NilDefaultsFalse(t *testing.T) {
	cfg := &Config{}
	if cfg.urlFromDOI() {
		t.Error("expected false when UrlFromDOI is nil")
	}
}

func TestUrlFromDOI_ExplicitTrue(t *testing.T) {
	v := true
	cfg := &Config{UrlFromDOI: &v}
	if !cfg.urlFromDOI() {
		t.Error("expected true when UrlFromDOI is &true")
	}
}

func TestUrlFromDOI_ExplicitFalse(t *testing.T) {
	v := false
	cfg := &Config{UrlFromDOI: &v}
	if cfg.urlFromDOI() {
		t.Error("expected false when UrlFromDOI is &false")
	}
}

func TestMaxAuthors_NilDefaultsUnlimited(t *testing.T) {
	cfg := &Config{}
	if cfg.maxAuthors() != 0 {
		t.Errorf("expected 0 (unlimited) when MaxAuthors is nil, got %d", cfg.maxAuthors())
	}
}

func TestMaxAuthors_ExplicitValue(t *testing.T) {
	v := 10
	cfg := &Config{MaxAuthors: &v}
	if cfg.maxAuthors() != 10 {
		t.Errorf("expected 10, got %d", cfg.maxAuthors())
	}
}

func TestMaxAuthors_ZeroValue(t *testing.T) {
	v := 0
	cfg := &Config{MaxAuthors: &v}
	if cfg.maxAuthors() != 0 {
		t.Errorf("expected 0 (unlimited), got %d", cfg.maxAuthors())
	}
}

func TestMaxAuthors_IEEEImpliesFive(t *testing.T) {
	trueVal := true
	cfg := &Config{IEEEFormat: &trueVal, MaxAuthors: nil}
	if cfg.maxAuthors() != 5 {
		t.Errorf("expected 5 when IEEEFormat is true and MaxAuthors is nil, got %d", cfg.maxAuthors())
	}
}

func TestMaxAuthors_ExplicitValueOverridesIEEE(t *testing.T) {
	trueVal := true
	v := 3
	cfg := &Config{IEEEFormat: &trueVal, MaxAuthors: &v}
	if cfg.maxAuthors() != 3 {
		t.Errorf("expected 3 (explicit value overrides IEEE), got %d", cfg.maxAuthors())
	}
}

// ── mergeConfig tests ────────────────────────────────────────────────────────

func TestMergeConfig_LocalOverridesGlobal(t *testing.T) {
	lv, gv := false, true
	local := &Config{AbbreviateJournals: &lv}
	global := &Config{AbbreviateJournals: &gv}
	merged := mergeConfig(local, global)
	if merged.AbbreviateJournals == nil || *merged.AbbreviateJournals != false {
		t.Errorf("expected local false to win, got %v", merged.AbbreviateJournals)
	}
}

func TestMergeConfig_GlobalFallback(t *testing.T) {
	gv := true
	local := &Config{}
	global := &Config{BraceTitles: &gv}
	merged := mergeConfig(local, global)
	if merged.BraceTitles == nil || *merged.BraceTitles != true {
		t.Errorf("expected global true, got %v", merged.BraceTitles)
	}
}

func TestMergeConfig_BothNil(t *testing.T) {
	merged := mergeConfig(&Config{}, &Config{})
	if merged.AbbreviateJournals != nil {
		t.Errorf("expected nil, got %v", merged.AbbreviateJournals)
	}
}

func TestMergeConfig_MainFromLocal(t *testing.T) {
	local := &Config{Main: "thesis.tex"}
	global := &Config{Main: "other.tex"}
	merged := mergeConfig(local, global)
	if merged.Main != "thesis.tex" {
		t.Errorf("Main = %q, want %q", merged.Main, "thesis.tex")
	}
}

func TestMergeConfig_IntField(t *testing.T) {
	gv := 5
	local := &Config{}
	global := &Config{MaxAuthors: &gv}
	merged := mergeConfig(local, global)
	if merged.MaxAuthors == nil || *merged.MaxAuthors != 5 {
		t.Errorf("expected global max-authors 5, got %v", merged.MaxAuthors)
	}
}

// ── el config set (local) ────────────────────────────────────────────────────

func TestConfigSet_BoolNoValue(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfigSet(t, []string{"abbreviate-journals"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := readConfig(t, dir)
	if cfg.AbbreviateJournals == nil || *cfg.AbbreviateJournals != true {
		t.Errorf("AbbreviateJournals = %v, want &true", cfg.AbbreviateJournals)
	}
}

func TestConfigSet_BoolTrue(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfigSet(t, []string{"brace-titles", "true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := readConfig(t, dir)
	if cfg.BraceTitles == nil || *cfg.BraceTitles != true {
		t.Errorf("BraceTitles = %v, want &true", cfg.BraceTitles)
	}
}

func TestConfigSet_BoolFalse(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfigSet(t, []string{"abbreviate-journals", "false"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := readConfig(t, dir)
	if cfg.AbbreviateJournals == nil || *cfg.AbbreviateJournals != false {
		t.Errorf("AbbreviateJournals = %v, want &false", cfg.AbbreviateJournals)
	}
}

func TestConfigSet_IntValue(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfigSet(t, []string{"max-authors", "10"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := readConfig(t, dir)
	if cfg.MaxAuthors == nil || *cfg.MaxAuthors != 10 {
		t.Errorf("MaxAuthors = %v, want &10", cfg.MaxAuthors)
	}
}

func TestConfigSet_IntZero(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfigSet(t, []string{"max-authors", "0"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := readConfig(t, dir)
	if cfg.MaxAuthors == nil || *cfg.MaxAuthors != 0 {
		t.Errorf("MaxAuthors = %v, want &0", cfg.MaxAuthors)
	}
}

func TestConfigSet_IntNegative(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	// Call runConfigSet directly to avoid cobra parsing "-1" as a flag.
	cmd := &cobra.Command{}
	cmd.Flags().Bool("global", false, "")
	if err := runConfigSet(cmd, []string{"max-authors", "-1"}); err == nil {
		t.Fatal("expected error for negative max-authors")
	}
}

func TestConfigSet_IntMissingValue(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfigSet(t, []string{"max-authors"}); err == nil {
		t.Fatal("expected error for max-authors without value")
	}
}

func TestConfigSet_InvalidBoolValue(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfigSet(t, []string{"brace-titles", "yes"}); err == nil {
		t.Fatal("expected error for invalid bool value")
	}
}

func TestConfigSet_UnknownKey(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfigSet(t, []string{"no-such-key"}); err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestConfigSet_PreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Main: "thesis.tex", BibFiles: []string{"refs.bib"}}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.MkdirAll(filepath.Join(dir, ".el"), 0755)
	os.WriteFile(filepath.Join(dir, ".el", "config.json"), data, 0644)
	chdir(t, dir)

	if err := invokeConfigSet(t, []string{"brace-titles", "true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	updated := readConfig(t, dir)
	if updated.Main != "thesis.tex" {
		t.Errorf("Main = %q, want %q", updated.Main, "thesis.tex")
	}
	if len(updated.BibFiles) != 1 || updated.BibFiles[0] != "refs.bib" {
		t.Errorf("BibFiles = %v, want [refs.bib]", updated.BibFiles)
	}
}

func TestConfigSet_NotInitialized(t *testing.T) {
	chdir(t, t.TempDir())
	if err := invokeConfigSet(t, []string{"brace-titles", "true"}); err == nil {
		t.Fatal("expected error when .el missing, got nil")
	}
}

// ── el config unset (local) ──────────────────────────────────────────────────

func TestConfigUnset_Bool(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	// First set it to true
	invokeConfigSet(t, []string{"brace-titles", "true"})

	// Unset should set to false
	if err := invokeConfigUnset(t, []string{"brace-titles"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := readConfig(t, dir)
	if cfg.BraceTitles == nil || *cfg.BraceTitles != false {
		t.Errorf("BraceTitles after unset = %v, want &false", cfg.BraceTitles)
	}
}

func TestConfigUnset_Int(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	// First set it
	invokeConfigSet(t, []string{"max-authors", "10"})

	// Unset should clear to nil
	if err := invokeConfigUnset(t, []string{"max-authors"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := readConfig(t, dir)
	if cfg.MaxAuthors != nil {
		t.Errorf("MaxAuthors after unset = %v, want nil", cfg.MaxAuthors)
	}
}

func TestConfigUnset_UnknownKey(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)

	if err := invokeConfigUnset(t, []string{"no-such-key"}); err == nil {
		t.Fatal("expected error for unknown key")
	}
}

// ── el config set/unset --global ─────────────────────────────────────────────

func TestConfigSet_Global(t *testing.T) {
	home := t.TempDir()
	setGlobalConfigDir(t, home)

	if err := invokeConfigSet(t, []string{"--global", "ieee-format", "true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := readGlobalConfig(t, home)
	if cfg.IEEEFormat == nil || *cfg.IEEEFormat != true {
		t.Errorf("global IEEEFormat = %v, want &true", cfg.IEEEFormat)
	}
}

func TestConfigSet_GlobalBoolNoValue(t *testing.T) {
	home := t.TempDir()
	setGlobalConfigDir(t, home)

	if err := invokeConfigSet(t, []string{"--global", "ieee-format"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := readGlobalConfig(t, home)
	if cfg.IEEEFormat == nil || *cfg.IEEEFormat != true {
		t.Errorf("global IEEEFormat = %v, want &true", cfg.IEEEFormat)
	}
}

func TestConfigUnset_Global(t *testing.T) {
	home := t.TempDir()
	setGlobalConfigDir(t, home)

	// Set then unset
	invokeConfigSet(t, []string{"--global", "ieee-format", "true"})
	if err := invokeConfigUnset(t, []string{"--global", "ieee-format"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := readGlobalConfig(t, home)
	if cfg.IEEEFormat == nil || *cfg.IEEEFormat != false {
		t.Errorf("global IEEEFormat after unset = %v, want &false", cfg.IEEEFormat)
	}
}

func TestConfigSet_GlobalOutsideProject(t *testing.T) {
	home := t.TempDir()
	setGlobalConfigDir(t, home)
	chdir(t, t.TempDir()) // not a project

	if err := invokeConfigSet(t, []string{"--global", "brace-titles"}); err != nil {
		t.Fatalf("expected global set to work outside project, got: %v", err)
	}
}

func TestConfigSet_GlobalNoFile(t *testing.T) {
	home := t.TempDir()
	setGlobalConfigDir(t, home)

	// First global set when no file exists yet
	if err := invokeConfigSet(t, []string{"--global", "retry-timeout", "false"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg := readGlobalConfig(t, home)
	if cfg.RetryTimeout == nil || *cfg.RetryTimeout != false {
		t.Errorf("RetryTimeout = %v, want &false", cfg.RetryTimeout)
	}
}

// ── el config (bare) ─────────────────────────────────────────────────────────

func invokeConfigCmd(t *testing.T, args []string) error {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().Bool("list", false, "")
	cmd.Flags().Bool("global", false, "")
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	return runConfigCmd(cmd, cmd.Flags().Args())
}

func TestConfigBare_Fails(t *testing.T) {
	if err := invokeConfigCmd(t, nil); err == nil {
		t.Fatal("expected error for bare el config")
	}
}

func TestConfigList_OutsideProject(t *testing.T) {
	chdir(t, t.TempDir())
	setGlobalConfigDir(t, t.TempDir())
	if err := invokeConfigCmd(t, []string{"--list"}); err != nil {
		t.Fatalf("expected --list to work outside project, got: %v", err)
	}
}

func TestConfigList_NoError(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "main.tex")
	chdir(t, dir)
	setGlobalConfigDir(t, t.TempDir())

	if err := invokeConfigCmd(t, []string{"--list"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigList_GlobalOnly(t *testing.T) {
	home := t.TempDir()
	setGlobalConfigDir(t, home)
	chdir(t, t.TempDir()) // not a project

	// Set a global value first
	invokeConfigSet(t, []string{"--global", "ieee-format", "true"})

	if err := invokeConfigCmd(t, []string{"--list", "--global"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigList_GlobalOutsideProject(t *testing.T) {
	home := t.TempDir()
	setGlobalConfigDir(t, home)
	chdir(t, t.TempDir()) // not a project

	// --list --global should work outside a project
	if err := invokeConfigCmd(t, []string{"--list", "--global"}); err != nil {
		t.Fatalf("expected to work outside project, got: %v", err)
	}
}

// ── configSource ─────────────────────────────────────────────────────────────

func TestConfigSource_Local(t *testing.T) {
	v := true
	local := &Config{BraceTitles: &v}
	global := &Config{}
	merged := mergeConfig(local, global)
	f := *findField("brace-titles")
	if s := configSource(f, local, global, merged); s != "(local)" {
		t.Errorf("source = %q, want %q", s, "(local)")
	}
}

func TestConfigSource_Global(t *testing.T) {
	v := true
	local := &Config{}
	global := &Config{BraceTitles: &v}
	merged := mergeConfig(local, global)
	f := *findField("brace-titles")
	if s := configSource(f, local, global, merged); s != "(global)" {
		t.Errorf("source = %q, want %q", s, "(global)")
	}
}

func TestConfigSource_Default(t *testing.T) {
	local := &Config{}
	global := &Config{}
	merged := mergeConfig(local, global)
	f := *findField("brace-titles")
	if s := configSource(f, local, global, merged); s != "(default)" {
		t.Errorf("source = %q, want %q", s, "(default)")
	}
}

func TestConfigSource_IEEEDefault(t *testing.T) {
	v := true
	local := &Config{IEEEFormat: &v}
	global := &Config{}
	merged := mergeConfig(local, global)
	f := *findField("max-authors")
	if s := configSource(f, local, global, merged); s != "(ieee default)" {
		t.Errorf("source = %q, want %q", s, "(ieee default)")
	}
}

// ── findField / validKeys ────────────────────────────────────────────────────

func TestFindField_Known(t *testing.T) {
	for _, name := range []string{"abbreviate-journals", "max-authors", "ieee-format"} {
		if findField(name) == nil {
			t.Errorf("findField(%q) = nil", name)
		}
	}
}

func TestFindField_Unknown(t *testing.T) {
	if findField("bogus") != nil {
		t.Error("findField(bogus) should be nil")
	}
}

func TestValidKeys_ContainsAll(t *testing.T) {
	keys := validKeys()
	for _, f := range configFields {
		if !strings.Contains(keys, f.key) {
			t.Errorf("validKeys() missing %q", f.key)
		}
	}
}
