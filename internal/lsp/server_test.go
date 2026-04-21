package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDetectCitePrefix(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		char   int
		prefix string
		ok     bool
	}{
		// standard
		{"cite", `\cite{foo`, 9, "foo", true},
		{"cite empty", `\cite{`, 6, "", true},
		{"cite multi", `\cite{foo,bar`, 13, "bar", true},
		// natbib basic
		{"citet", `\citet{key`, 10, "key", true},
		{"citep", `\citep{key`, 10, "key", true},
		{"citealt", `\citealt{key`, 12, "key", true},
		{"citealp", `\citealp{key`, 12, "key", true},
		{"citeauthor", `\citeauthor{key`, 15, "key", true},
		{"citeyear", `\citeyear{key`, 13, "key", true},
		{"citeyearpar", `\citeyearpar{key`, 16, "key", true},
		{"citenum", `\citenum{key`, 12, "key", true},
		// capitalised
		{"Citet", `\Citet{key`, 10, "key", true},
		{"Citep", `\Citep{key`, 10, "key", true},
		{"Citealt", `\Citealt{key`, 12, "key", true},
		{"Citealp", `\Citealp{key`, 12, "key", true},
		{"Citeauthor", `\Citeauthor{key`, 15, "key", true},
		// starred
		{"citet*", `\citet*{key`, 11, "key", true},
		{"citep*", `\citep*{key`, 11, "key", true},
		{"citealt*", `\citealt*{key`, 13, "key", true},
		{"Citet*", `\Citet*{key`, 11, "key", true},
		// optional args
		{"citep opt", `\citep[see][p.1]{key`, 20, "key", true},
		{"citet opt", `\citet[e.g.]{key`, 16, "key", true},
		{"Citep opt", `\Citep[see][]{key`, 17, "key", true},
		// no match
		{"no cite", `hello world`, 11, "", false},
		{"closed brace", `\cite{key}`, 10, "", false},
		{"citetext", `\citetext{foo`, 13, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, ok := detectCitePrefix(tt.line, tt.char)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if prefix != tt.prefix {
				t.Fatalf("prefix = %q, want %q", prefix, tt.prefix)
			}
		})
	}
}

func TestReadMessage(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantMethod string
		wantErr   bool
	}{
		{
			name:       "valid message",
			body:       `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
			wantMethod: "initialize",
			wantErr:    false,
		},
		{
			name:       "message with extra headers",
			body:       `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
			wantMethod: "shutdown",
			wantErr:    false,
		},
		{
			name:    "missing content-length",
			body:    "",
			wantErr: true,
		},
		{
			name:    "zero content-length",
			body:    "",
			wantErr: true,
		},
		{
			name:    "incomplete body",
			body:    `{"jsonrpc":"2.0"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input string
			switch tt.name {
			case "valid message":
				input = "Content-Length: " + formatInt(len(tt.body)) + "\r\n\r\n" + tt.body
			case "message with extra headers":
				input = "Content-Length: " + formatInt(len(tt.body)) + "\r\nContent-Type: application/json\r\n\r\n" + tt.body
			case "missing content-length":
				input = "\r\n" + tt.body
			case "zero content-length":
				input = "Content-Length: 0\r\n\r\n"
			case "incomplete body":
				input = "Content-Length: 50\r\n\r\n" + tt.body
			}
			br := bufio.NewReader(strings.NewReader(input))
			req, err := readMessage(br)
			if (err != nil) != tt.wantErr {
				t.Fatalf("readMessage error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if req == nil {
				t.Fatal("readMessage returned nil request")
			}
			if req.Method != tt.wantMethod {
				t.Fatalf("Method = %q, want %q", req.Method, tt.wantMethod)
			}
		})
	}
}

func TestWriteMessage(t *testing.T) {
	tests := []struct {
		name  string
		value any
		check func(t *testing.T, output string)
	}{
		{
			name: "simple response",
			value: response{
				JSONRPC: "2.0",
				ID:      json.RawMessage("1"),
				Result:  nil,
			},
			check: func(t *testing.T, output string) {
				if !strings.HasPrefix(output, "Content-Length: ") {
					t.Fatal("missing Content-Length header")
				}
				if !strings.Contains(output, "\r\n\r\n") {
					t.Fatal("missing \\r\\n\\r\\n separator")
				}
				parts := strings.Split(output, "\r\n\r\n")
				if len(parts) != 2 {
					t.Fatalf("expected 2 parts, got %d", len(parts))
				}
				body := parts[1]
				if !strings.Contains(body, `"jsonrpc":"2.0"`) {
					t.Fatal("body missing jsonrpc field")
				}
			},
		},
		{
			name: "completion list response",
			value: response{
				JSONRPC: "2.0",
				ID:      json.RawMessage("42"),
				Result: completionList{
					IsIncomplete: false,
					Items: []completionItem{
						{Label: "key1"},
						{Label: "key2"},
					},
				},
			},
			check: func(t *testing.T, output string) {
				parts := strings.Split(output, "\r\n\r\n")
				body := parts[1]
				var result completionList
				var resp response
				resp.Result = &result
				if err := json.Unmarshal([]byte(body), &resp); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if len(result.Items) != 2 {
					t.Fatalf("expected 2 items, got %d", len(result.Items))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			writeMessage(buf, tt.value)
			output := buf.String()
			tt.check(t, output)
		})
	}
}

func TestLineAt(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		line     int
		expected string
	}{
		{
			name:     "single line - line 0",
			text:     "hello world",
			line:     0,
			expected: "hello world",
		},
		{
			name:     "first line of multi-line",
			text:     "line 0\nline 1\nline 2",
			line:     0,
			expected: "line 0",
		},
		{
			name:     "middle line",
			text:     "line 0\nline 1\nline 2",
			line:     1,
			expected: "line 1",
		},
		{
			name:     "last line",
			text:     "line 0\nline 1\nline 2",
			line:     2,
			expected: "line 2",
		},
		{
			name:     "line past end",
			text:     "line 0\nline 1",
			line:     5,
			expected: "",
		},
		{
			name:     "empty text",
			text:     "",
			line:     0,
			expected: "",
		},
		{
			name:     "empty line in middle",
			text:     "line 0\n\nline 2",
			line:     1,
			expected: "",
		},
		{
			name:     "trailing newline",
			text:     "line 0\nline 1\n",
			line:     1,
			expected: "line 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lineAt(tt.text, tt.line)
			if result != tt.expected {
				t.Fatalf("lineAt(%q, %d) = %q, want %q", tt.text, tt.line, result, tt.expected)
			}
		})
	}
}

func TestServerComplete(t *testing.T) {
	tests := []struct {
		name     string
		items    []completionItem
		text     string
		line     int
		char     int
		expected int // number of expected items
		checkLabels func(t *testing.T, items []completionItem)
	}{
		{
			name: "no cite trigger",
			items: []completionItem{
				{Label: "foo"},
				{Label: "bar"},
			},
			text:     "hello world",
			line:     0,
			char:     11,
			expected: 0,
		},
		{
			name: "cite with no prefix returns all",
			items: []completionItem{
				{Label: "alpha"},
				{Label: "beta"},
				{Label: "gamma"},
			},
			text:     "\\cite{",
			line:     0,
			char:     6,
			expected: 3,
		},
		{
			name: "cite with matching prefix",
			items: []completionItem{
				{Label: "apple"},
				{Label: "apricot"},
				{Label: "banana"},
			},
			text:     "\\cite{ap",
			line:     0,
			char:     8,
			expected: 2,
			checkLabels: func(t *testing.T, items []completionItem) {
				labels := make(map[string]bool)
				for _, item := range items {
					labels[item.Label] = true
				}
				if !labels["apple"] || !labels["apricot"] {
					t.Fatalf("expected apple and apricot, got %v", labels)
				}
				if labels["banana"] {
					t.Fatal("unexpected banana in results")
				}
			},
		},
		{
			name: "cite with no matches",
			items: []completionItem{
				{Label: "foo"},
				{Label: "bar"},
			},
			text:     "\\cite{xyz",
			line:     0,
			char:     9,
			expected: 0,
		},
		{
			name: "multi-line text with cite on second line",
			items: []completionItem{
				{Label: "reference1"},
				{Label: "reference2"},
			},
			text:     "some text\n\\cite{ref",
			line:     1,
			char:     10,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &server{
				docs:  make(map[string]string),
				items: tt.items,
			}
			s.docs["test.tex"] = tt.text

			p := completionParams{
				TextDocument: textDocumentIdentifier{URI: "test.tex"},
				Position:     position{Line: tt.line, Character: tt.char},
			}

			result := s.complete(p)
			if len(result) != tt.expected {
				t.Fatalf("complete returned %d items, want %d", len(result), tt.expected)
			}

			if tt.checkLabels != nil {
				tt.checkLabels(t, result)
			}
		})
	}
}

func TestServeFullCycle(t *testing.T) {
	// Test a full request/response cycle via Serve
	items := []completionItem{
		{Label: "testkey1"},
		{Label: "testkey2"},
	}

	// Create input: initialize request followed by exit
	id1 := json.RawMessage("1")
	initReq := request{
		JSONRPC: "2.0",
		ID:      &id1,
		Method:  "initialize",
		Params:  json.RawMessage("{}"),
	}
	initData, _ := json.Marshal(initReq)
	initMsg := "Content-Length: " + formatInt(len(initData)) + "\r\n\r\n" + string(initData)

	id2 := json.RawMessage("2")
	exitReq := request{
		JSONRPC: "2.0",
		ID:      &id2,
		Method:  "shutdown",
		Params:  json.RawMessage("{}"),
	}
	exitData, _ := json.Marshal(exitReq)
	exitMsg := "Content-Length: " + formatInt(len(exitData)) + "\r\n\r\n" + string(exitData)

	exitNotif := request{
		JSONRPC: "2.0",
		Method:  "exit",
		Params:  json.RawMessage("{}"),
	}
	exitNotifData, _ := json.Marshal(exitNotif)
	exitNotifMsg := "Content-Length: " + formatInt(len(exitNotifData)) + "\r\n\r\n" + string(exitNotifData)

	input := strings.NewReader(initMsg + exitMsg + exitNotifMsg)
	output := &bytes.Buffer{}

	// Run Serve (it will call os.Exit, so we can't test the exit path)
	// Instead, we'll test the parts that don't exit
	br := bufio.NewReader(input)
	s := &server{
		docs:  make(map[string]string),
		items: items,
		w:     output,
	}

	// Read and handle initialize
	req, err := readMessage(br)
	if err != nil {
		t.Fatalf("readMessage failed: %v", err)
	}
	if req.Method != "initialize" {
		t.Fatalf("expected initialize, got %s", req.Method)
	}

	stop := s.handle(req)
	if stop {
		t.Fatal("handle returned true unexpectedly")
	}

	// Check that response was written
	outStr := output.String()
	if !strings.Contains(outStr, "Content-Length:") {
		t.Fatal("no response written")
	}
	if !strings.Contains(outStr, `"textDocumentSync"`) {
		t.Fatal("response missing textDocumentSync capability")
	}

	// Read and handle shutdown
	req, err = readMessage(br)
	if err != nil {
		t.Fatalf("readMessage failed: %v", err)
	}
	if req.Method != "shutdown" {
		t.Fatalf("expected shutdown, got %s", req.Method)
	}

	stop = s.handle(req)
	if stop {
		t.Fatal("handle returned true unexpectedly")
	}
	if !s.shuttingDown {
		t.Fatal("shuttingDown not set after shutdown request")
	}
}

func TestHandleDidOpen(t *testing.T) {
	s := &server{
		docs:  make(map[string]string),
		items: []completionItem{},
	}

	params := didOpenParams{
		TextDocument: textDocumentItem{
			URI:  "file:///test.tex",
			Text: "\\documentclass{article}\n\\begin{document}\n\\end{document}",
		},
	}
	paramsData, _ := json.Marshal(params)

	req := &request{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params:  paramsData,
	}

	s.handle(req)

	if text, ok := s.docs["file:///test.tex"]; !ok {
		t.Fatal("document not stored")
	} else if text != params.TextDocument.Text {
		t.Fatalf("stored text doesn't match: %q", text)
	}
}

func TestHandleDidChange(t *testing.T) {
	s := &server{
		docs:  make(map[string]string),
		items: []completionItem{},
	}

	// Store initial document
	s.docs["file:///test.tex"] = "old content"

	params := didChangeParams{
		TextDocument: versionedTextDocumentIdentifier{
			URI: "file:///test.tex",
		},
		ContentChanges: []textDocumentContentChangeEvent{
			{Text: "updated content"},
		},
	}
	paramsData, _ := json.Marshal(params)

	req := &request{
		JSONRPC: "2.0",
		Method:  "textDocument/didChange",
		Params:  paramsData,
	}

	s.handle(req)

	if text := s.docs["file:///test.tex"]; text != "updated content" {
		t.Fatalf("document not updated: %q", text)
	}
}

func TestHandleCompletionRequest(t *testing.T) {
	items := []completionItem{
		{Label: "key1"},
		{Label: "key2"},
	}
	s := &server{
		docs:  make(map[string]string),
		items: items,
		w:     &bytes.Buffer{},
	}

	// Store document with cite
	s.docs["file:///test.tex"] = "\\cite{ke"

	params := completionParams{
		TextDocument: textDocumentIdentifier{URI: "file:///test.tex"},
		Position:     position{Line: 0, Character: 8},
	}
	paramsData, _ := json.Marshal(params)

	id := json.RawMessage("100")
	req := &request{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "textDocument/completion",
		Params:  paramsData,
	}

	stop := s.handle(req)
	if stop {
		t.Fatal("handle returned true")
	}

	output := s.w.(*bytes.Buffer).String()
	if !strings.Contains(output, "key1") {
		t.Fatal("response missing key1")
	}
	if !strings.Contains(output, "key2") {
		t.Fatal("response missing key2")
	}
}

func TestHandleMethodNotFound(t *testing.T) {
	s := &server{
		docs:  make(map[string]string),
		items: []completionItem{},
		w:     &bytes.Buffer{},
	}

	id := json.RawMessage("1")
	req := &request{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "unknownMethod",
	}

	s.handle(req)

	output := s.w.(*bytes.Buffer).String()
	if !strings.Contains(output, "method not found") {
		t.Fatalf("response missing error message: %s", output)
	}
}

// Helper to format int as string
func formatInt(n int) string {
	b := make([]byte, 0, 10)
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if len(b) == 0 {
		return "0"
	}
	return string(b)
}
