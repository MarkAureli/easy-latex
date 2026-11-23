package lsp

import (
	"os"
	"path/filepath"
	"time"

	"github.com/MarkAureli/easy-latex/internal/pedantic"
	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// cmdTable is a cached overlay of one project's .el/cmds.json (command
// arities) and .el/mathclass.json (symbol math classes).
type cmdTable struct {
	argsMtime, classMtime time.Time
	ctx                   *pedantic.Context
}

func mtimeOf(path string) time.Time {
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

// checkContext returns the pedantic context for the file at path: built-in
// command arities and math classes overlaid by the project's .el/cmds.json
// and .el/mathclass.json, re-read only when a file's mtime changes. Unlike el
// check, no project files are scanned (too costly per keystroke); the
// on-save refresh keeps declarations in cmds.json, and commands unknown until
// then are simply left alone.
func (s *server) checkContext(path string) *pedantic.Context {
	root := projectRoot(filepath.Dir(path))
	if root == "" {
		return &pedantic.Context{CmdArgs: texscan.BuiltinCmdArgs, MathClasses: texscan.BuiltinMathClasses}
	}
	argsPath := filepath.Join(root, ".el", "cmds.json")
	classPath := filepath.Join(root, ".el", "mathclass.json")
	am, cm := mtimeOf(argsPath), mtimeOf(classPath)
	if t, ok := s.cmdTables[root]; ok && t.argsMtime.Equal(am) && t.classMtime.Equal(cm) {
		return t.ctx
	}
	if s.cmdTables == nil {
		s.cmdTables = map[string]cmdTable{}
	}
	t := cmdTable{argsMtime: am, classMtime: cm, ctx: &pedantic.Context{
		CmdArgs:     texscan.StoredCmdArgs(argsPath),
		MathClasses: texscan.StoredMathClasses(classPath),
	}}
	s.cmdTables[root] = t
	return t.ctx
}
