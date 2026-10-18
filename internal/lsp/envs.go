package lsp

import (
	"maps"
	"os"
	"path/filepath"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

// refreshEnvTable rescans declarations in the project containing uri and
// updates .el/envs.json (theorem flags, declared argument counts). Usage-derived
// argument counts stay compile-gated. Best effort; no-op outside an el project.
func refreshEnvTable(uri string) {
	root := projectRoot(filepath.Dir(uriToPath(uri)))
	if root == "" {
		return
	}
	files := texscan.FindFilesByExt(root, ".tex", ".sty", ".cls")
	args, theorems := texscan.ScanEnvDecls(files)

	path := filepath.Join(root, ".el", "envs.json")
	table := texscan.LoadEnvTable(path)
	before := maps.Clone(table)
	texscan.ApplyEnvDecls(table, args, theorems)
	_ = texscan.SaveEnvTable(path, table, before)
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
