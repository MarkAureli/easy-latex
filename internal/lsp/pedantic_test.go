package lsp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPublishDiagnostics_FlagsViolation(t *testing.T) {
	buf := &bytes.Buffer{}
	s := &server{
		docs:          map[string]string{},
		enabledChecks: []string{"single-spaces"},
		w:             buf,
	}
	s.docs["file:///t.tex"] = "Hello  world.\n"
	s.publishDiagnostics("file:///t.tex")

	out := buf.String()
	if !strings.Contains(out, "textDocument/publishDiagnostics") {
		t.Fatal("expected publishDiagnostics notification, got: " + out)
	}
	if !strings.Contains(out, "el-pedantic") {
		t.Fatal("expected source=el-pedantic")
	}
	if !strings.Contains(out, "consecutive spaces") {
		t.Fatal("expected message about consecutive spaces")
	}
}

func TestPublishDiagnostics_CleanFile(t *testing.T) {
	buf := &bytes.Buffer{}
	s := &server{
		docs:          map[string]string{"file:///t.tex": "clean line\n"},
		enabledChecks: []string{"single-spaces"},
		w:             buf,
	}
	s.publishDiagnostics("file:///t.tex")

	// Should still notify (with empty diagnostics) so editor clears stale state.
	out := buf.String()
	if !strings.Contains(out, "publishDiagnostics") {
		t.Fatal("expected notification even when clean")
	}
	if !strings.Contains(out, `"diagnostics":[]`) {
		t.Fatalf("expected empty diagnostics array, got: %s", out)
	}
}

func TestPublishDiagnostics_NoChecksEnabled(t *testing.T) {
	buf := &bytes.Buffer{}
	s := &server{
		docs:          map[string]string{"file:///t.tex": "Hello  world.\n"},
		enabledChecks: nil,
		w:             buf,
	}
	s.publishDiagnostics("file:///t.tex")

	if buf.Len() != 0 {
		t.Fatalf("expected no output when no checks enabled, got: %s", buf.String())
	}
}

func TestCodeActions_FixAvailable(t *testing.T) {
	s := &server{
		docs:          map[string]string{"file:///t.tex": "Hello  world.\n"},
		enabledChecks: []string{"single-spaces"},
	}
	actions := s.codeActions(codeActionParams{
		TextDocument: textDocumentIdentifier{URI: "file:///t.tex"},
	})
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Kind != "quickfix" {
		t.Errorf("kind = %q, want quickfix", a.Kind)
	}
	if a.Edit == nil {
		t.Fatal("edit is nil")
	}
	edits := a.Edit.Changes["file:///t.tex"]
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if !strings.Contains(edits[0].NewText, "Hello world.") {
		t.Errorf("newText = %q, want corrected", edits[0].NewText)
	}
}

func TestCodeActions_NoFixWhenClean(t *testing.T) {
	s := &server{
		docs:          map[string]string{"file:///t.tex": "clean line\n"},
		enabledChecks: []string{"single-spaces"},
	}
	actions := s.codeActions(codeActionParams{
		TextDocument: textDocumentIdentifier{URI: "file:///t.tex"},
	})
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for clean doc, got %d", len(actions))
	}
}

func TestUriToPath(t *testing.T) {
	cases := map[string]string{
		"file:///foo/bar.tex":         "/foo/bar.tex",
		"file:///Users/me/main.tex":   "/Users/me/main.tex",
		"untitled:Untitled-1":         "untitled:Untitled-1",
		"":                            "",
	}
	for in, want := range cases {
		if got := uriToPath(in); got != want {
			t.Errorf("uriToPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHandleDidOpen_TriggersPublishDiagnostics(t *testing.T) {
	buf := &bytes.Buffer{}
	s := &server{
		docs:          map[string]string{},
		enabledChecks: []string{"single-spaces"},
		w:             buf,
	}
	params := didOpenParams{
		TextDocument: textDocumentItem{
			URI:  "file:///t.tex",
			Text: "Hello  world.\n",
		},
	}
	data, _ := json.Marshal(params)
	s.handle(&request{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params:  data,
	})
	if !strings.Contains(buf.String(), "publishDiagnostics") {
		t.Fatal("expected publishDiagnostics on didOpen")
	}
}
