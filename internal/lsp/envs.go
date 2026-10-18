package lsp

import (
	"maps"
	"os"
	"path/filepath"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// refreshEnvTable rescans declarations in the project containing uri and
// updates .el/envs.json (theorem flags, declared argument counts) and the
// declared entries of .el/cmds.json. Usage-derived argument counts stay
// compile-gated. Best effort; no-op outside an el project.
func refreshEnvTable(uri string) {
	root := projectRoot(filepath.Dir(uriToPath(uri)))
	if root == "" {
		return
	}
	files := texscan.FindFilesByExt(root, ".tex", ".sty", ".cls")
	args, theorems := texscan.ScanEnvDecls(files)

	path := filepath.Join(root, ".el", "envs.json")
	table := texscan.LoadTable[texscan.EnvInfo](path)
	before := maps.Clone(table)
	texscan.ApplyEnvDecls(table, args, theorems)
	_ = texscan.SaveTable(path, table, before)

	cmdPath := filepath.Join(root, ".el", "cmds.json")
	cmds := texscan.LoadTable[int](cmdPath)
	cmdsBefore := maps.Clone(cmds)
	maps.Copy(cmds, texscan.ScanCmdDecls(files))
	_ = texscan.SaveTable(cmdPath, cmds, cmdsBefore)
}

// projectRoot returns the nearest ancestor of dir containing a .el directory.
func projectRoot(dir string) string {
	for {
		if info, err := os.Stat(filepath.Join(dir, ".el")); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
