package lsp

import (
	"os"
	"path/filepath"
	"time"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// cmdTable is a cached command-arity overlay of one project's .el/cmds.json.
type cmdTable struct {
	mtime time.Time
	args  map[string]int
}

// checkContext returns the pedantic context for the file at path: built-in
// command arities overlaid by the project's .el/cmds.json, re-read only when
// that file's mtime changes. Unlike el check, no project files are scanned
// (too costly per keystroke); the on-save refresh keeps declarations in
// cmds.json, and commands unknown until then are simply left alone.
func (s *server) checkContext(path string) *pedantic.Context {
	root := projectRoot(filepath.Dir(path))
	if root == "" {
		return &pedantic.Context{CmdArgs: texscan.BuiltinCmdArgs}
	}
	cmdPath := filepath.Join(root, ".el", "cmds.json")
	var mtime time.Time
	if info, err := os.Stat(cmdPath); err == nil {
		mtime = info.ModTime()
	}
	if t, ok := s.cmdTables[root]; ok && t.mtime.Equal(mtime) {
		return &pedantic.Context{CmdArgs: t.args}
	}
	if s.cmdTables == nil {
		s.cmdTables = map[string]cmdTable{}
	}
	t := cmdTable{mtime: mtime, args: texscan.StoredCmdArgs(cmdPath)}
	s.cmdTables[root] = t
	return &pedantic.Context{CmdArgs: t.args}
}
