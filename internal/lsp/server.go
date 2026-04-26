package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
)

const codeMethodNotFound = -32601

// Config carries optional integration points for the server.
type Config struct {
	Items         []completionItem // cite-key items
	EnabledChecks []string         // static pedantic check names; empty = no linting
}

// Serve starts the LSP server, reading JSON-RPC from r and writing to w.
func Serve(cfg Config, r io.Reader, w io.Writer) error {
	s := &server{
		docs:          make(map[string]string),
		items:         cfg.Items,
		enabledChecks: cfg.EnabledChecks,
		w:             w,
	}
	br := bufio.NewReader(r)
	for {
		req, err := readMessage(br)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if stop := s.handle(req); stop {
			return nil
		}
	}
}

type server struct {
	docs          map[string]string // uri → full text
	items         []completionItem
	enabledChecks []string
	w             io.Writer
	shuttingDown  bool
}

func (s *server) handle(req *request) (stop bool) {
	switch req.Method {
	case "initialize":
		s.reply(req.ID, initializeResult{
			Capabilities: serverCapabilities{
				TextDocumentSync: 1, // full
				CompletionProvider: &completionOptions{
					TriggerCharacters: []string{"{", ","},
				},
				CodeActionProvider: len(s.enabledChecks) > 0,
			},
		})

	case "initialized":
		// notification, no response

	case "textDocument/didOpen":
		var p didOpenParams
		if err := json.Unmarshal(req.Params, &p); err == nil {
			s.docs[p.TextDocument.URI] = p.TextDocument.Text
			s.publishDiagnostics(p.TextDocument.URI)
		}

	case "textDocument/didChange":
		var p didChangeParams
		if err := json.Unmarshal(req.Params, &p); err == nil && len(p.ContentChanges) > 0 {
			s.docs[p.TextDocument.URI] = p.ContentChanges[len(p.ContentChanges)-1].Text
			s.publishDiagnostics(p.TextDocument.URI)
		}

	case "textDocument/codeAction":
		var p codeActionParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			s.replyError(req.ID, codeMethodNotFound, err.Error())
			return false
		}
		s.reply(req.ID, s.codeActions(p))

	case "textDocument/completion":
		var p completionParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			s.replyError(req.ID, codeMethodNotFound, err.Error())
			return false
		}
		items := s.complete(p)
		s.reply(req.ID, completionList{IsIncomplete: false, Items: items})

	case "shutdown":
		s.shuttingDown = true
		s.reply(req.ID, nil)

	case "exit":
		if s.shuttingDown {
			os.Exit(0)
		}
		os.Exit(1)

	default:
		// Only respond to requests (have an ID), not notifications.
		if req.ID != nil {
			s.replyError(req.ID, codeMethodNotFound, "method not found: "+req.Method)
		}
	}
	return false
}

// publishDiagnostics runs static pedantic checks on the document at uri and
// pushes a textDocument/publishDiagnostics notification to the client. Always
// emits, even when empty, so previously-shown diagnostics clear.
func (s *server) publishDiagnostics(uri string) {
	if len(s.enabledChecks) == 0 {
		return
	}
	text, ok := s.docs[uri]
	if !ok {
		return
	}
	path := uriToPath(uri)
	pedDiags := pedantic.RunSourceChecksText(s.enabledChecks, path, text)
	lspDiags := make([]lspDiagnostic, 0, len(pedDiags))
	rawLines := strings.Split(text, "\n")
	for _, d := range pedDiags {
		line := d.Line - 1
		if line < 0 {
			line = 0
		}
		end := 0
		if line < len(rawLines) {
			end = len(rawLines[line])
		}
		lspDiags = append(lspDiags, lspDiagnostic{
			Range: lspRange{
				Start: position{Line: line, Character: 0},
				End:   position{Line: line, Character: end},
			},
			Severity: 2, // Warning
			Source:   "el-pedantic",
			Message:  d.Message,
		})
	}
	s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         uri,
		Diagnostics: lspDiags,
	})
}

// codeActions returns autofix actions for the requested range. Currently a
// single "Apply pedantic autofix" action that runs all enabled fixable checks
// over the document and returns the result as one whole-document text edit.
func (s *server) codeActions(p codeActionParams) []codeAction {
	if len(s.enabledChecks) == 0 {
		return nil
	}
	if !pedantic.HasFixableChecks(s.enabledChecks) {
		return nil
	}
	text, ok := s.docs[p.TextDocument.URI]
	if !ok {
		return nil
	}
	path := uriToPath(p.TextDocument.URI)
	newText, changed := pedantic.RunSourceFixesText(s.enabledChecks, path, text)
	if !changed {
		return nil
	}
	return []codeAction{{
		Title:       "Apply pedantic autofix",
		Kind:        "quickfix",
		IsPreferred: true,
		Edit: &workspaceEdit{
			Changes: map[string][]textEdit{
				p.TextDocument.URI: {{
					Range:   wholeDocRange(text),
					NewText: newText,
				}},
			},
		},
	}}
}

// notify sends a server→client JSON-RPC notification.
func (s *server) notify(method string, params any) {
	writeMessage(s.w, notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}

// uriToPath converts a file:// URI to a local path. Returns the URI unchanged
// when no scheme is present (best-effort).
func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	return u.Path
}

// wholeDocRange returns a range covering the entire document text.
func wholeDocRange(text string) lspRange {
	lines := strings.Split(text, "\n")
	last := len(lines) - 1
	return lspRange{
		Start: position{Line: 0, Character: 0},
		End:   position{Line: last, Character: len(lines[last])},
	}
}

// complete returns filtered cite-key completions for the cursor position.
func (s *server) complete(p completionParams) []completionItem {
	text := s.docs[p.TextDocument.URI]
	line := lineAt(text, p.Position.Line)
	prefix, ok := detectCitePrefix(line, p.Position.Character)
	if !ok {
		return nil
	}
	if prefix == "" {
		return s.items
	}
	var out []completionItem
	for _, item := range s.items {
		if strings.HasPrefix(item.Label, prefix) {
			out = append(out, item)
		}
	}
	return out
}

// reCiteTrigger matches a cite command open brace with any content up to
// (but not including) the closing brace. Covers standard \cite plus natbib
// commands: \citet, \citep, \citealt, \citealp, \citeauthor, \citeyear,
// \citeyearpar, \citenum, capitalised \Cite* variants, and starred forms.
// Optional [...] arguments before the brace are skipped.
var reCiteTrigger = regexp.MustCompile(`\\[Cc]ite(?:t|p|alt|alp|author|year(?:par)?|num)?\*?(?:\[[^\]]*\])*\{[^}]*$`)

// detectCitePrefix returns the partially-typed key at cursor and true when
// the cursor is inside a \cite{...} argument. prefix="" means show all items.
func detectCitePrefix(line string, char int) (string, bool) {
	if char > len(line) {
		char = len(line)
	}
	sub := line[:char]
	if !reCiteTrigger.MatchString(sub) {
		return "", false
	}
	// Partial key = text after the last '{' or ','
	idx := strings.LastIndexAny(sub, "{,")
	if idx < 0 {
		return "", true
	}
	return strings.TrimSpace(sub[idx+1:]), true
}

// lineAt returns the zero-indexed line from text.
func lineAt(text string, line int) string {
	n := 0
	start := 0
	for i, c := range text {
		if c == '\n' {
			if n == line {
				return text[start:i]
			}
			n++
			start = i + 1
		}
	}
	if n == line {
		return text[start:]
	}
	return ""
}

// reply sends a successful JSON-RPC response. id may be nil for notifications
// that somehow need a response (shouldn't happen, but guard anyway).
func (s *server) reply(id *json.RawMessage, result any) {
	if id == nil {
		return
	}
	resp := response{
		JSONRPC: "2.0",
		ID:      *id,
		Result:  result,
	}
	writeMessage(s.w, resp)
}

func (s *server) replyError(id *json.RawMessage, code int, msg string) {
	if id == nil {
		return
	}
	resp := response{
		JSONRPC: "2.0",
		ID:      *id,
		Error:   &responseError{Code: code, Message: msg},
	}
	writeMessage(s.w, resp)
}

func readMessage(r *bufio.Reader) (*request, error) {
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if val, ok := strings.CutPrefix(line, "Content-Length: "); ok {
			n, err := strconv.Atoi(val)
			if err == nil {
				contentLength = n
			}
		}
	}
	if contentLength == 0 {
		return nil, fmt.Errorf("missing or zero Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func writeMessage(w io.Writer, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(data), data)
}
