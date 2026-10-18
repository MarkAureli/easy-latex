package cmd

import (
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/MarkAureli/easy-latex/internal/envprobe"
	"github.com/MarkAureli/easy-latex/internal/texscan"
)

const envsFile = "envs.json"

// scanProjectEnvs scans the project's tex, sty and cls files for environments.
func scanProjectEnvs(mainTex string) map[string]texscan.EnvInfo {
	files := append(texscan.FindTexFiles(mainTex, "."), texscan.FindFilesByExt(".", ".sty", ".cls")...)
	return texscan.ScanEnvs(files)
}

// writeEnvProbe writes the probe package for the scanned environments into the
// aux dir so the compile can inject it.
func writeEnvProbe(scanned map[string]texscan.EnvInfo) error {
	names := slices.Sorted(maps.Keys(scanned))
	return os.WriteFile(filepath.Join(auxDir, envprobe.PkgName+".sty"), envprobe.Sty(names), 0644)
}

// updateEnvTable refreshes .el/envs.json (environment name -> argument count,
// theorem/float/list flags) after a successful compile. Scanned entries
// overwrite stored ones, and probe verdicts (TeX's own state during the
// compile) set the float and list flags. Environments no longer
// present are kept.
func updateEnvTable(stem string, scanned map[string]texscan.EnvInfo) error {
	path := filepath.Join(auxDir, envsFile)
	table := texscan.LoadEnvTable(path)
	before := maps.Clone(table)
	for name, res := range envprobe.Parse(filepath.Join(auxDir, stem+envprobe.Ext)) {
		info := scanned[name]
		info.Float, info.List = res.Float, res.List
		scanned[name] = info
	}
	maps.Copy(table, scanned)
	return texscan.SaveEnvTable(path, table, before)
}
