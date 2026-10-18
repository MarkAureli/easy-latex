package cmd

import (
	"maps"
	"path/filepath"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

const envsFile = "envs.json"

// updateEnvTable refreshes .el/envs.json (environment name -> mandatory
// argument count and theorem flag) from the project's tex files. Entries found
// in the sources overwrite stored ones; environments no longer present are
// kept.
func updateEnvTable(mainTex string) error {
	path := filepath.Join(auxDir, envsFile)
	table := texscan.LoadEnvTable(path)
	before := maps.Clone(table)
	files := append(texscan.FindTexFiles(mainTex, "."), texscan.FindFilesByExt(".", ".sty", ".cls")...)
	maps.Copy(table, texscan.ScanEnvs(files))
	return texscan.SaveEnvTable(path, table, before)
}
