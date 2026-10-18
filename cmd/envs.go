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

const cmdsFile = "cmds.json"

// projectTexFiles lists the project's tex files reachable from mainTex plus all
// sty and cls files.
func projectTexFiles(mainTex string) []string {
	return append(texscan.FindTexFiles(mainTex, "."), texscan.FindFilesByExt(".", ".sty", ".cls")...)
}

// scanProjectEnvs scans the project's tex, sty and cls files for environments.
func scanProjectEnvs(mainTex string) map[string]texscan.EnvInfo {
	return texscan.ScanEnvs(projectTexFiles(mainTex))
}

// updateCmdTable refreshes .el/cmds.json (command name -> mandatory argument
// count) after a successful compile. Scanned entries overwrite stored ones;
// commands no longer present are kept.
func updateCmdTable(mainTex string) error {
	path := filepath.Join(auxDir, cmdsFile)
	table := texscan.LoadTable[int](path)
	before := maps.Clone(table)
	maps.Copy(table, texscan.ScanCmds(projectTexFiles(mainTex)))
	return texscan.SaveTable(path, table, before)
}

// writeEnvProbe writes the probe package for the scanned environments into the
// aux dir so the compile can inject it.
func writeEnvProbe(scanned map[string]texscan.EnvInfo) error {
	names := slices.Sorted(maps.Keys(scanned))
	return os.WriteFile(filepath.Join(auxDir, envprobe.PkgName+".sty"), envprobe.Sty(names), 0644)
}

// updateEnvTable refreshes .el/envs.json (environment name -> argument count,
// theorem/float flags) after a successful compile. Scanned entries
// overwrite stored ones, and probe verdicts (TeX's own state during the
// compile) set the float flag. Environments no longer
// present are kept.
func updateEnvTable(stem string, scanned map[string]texscan.EnvInfo) error {
	path := filepath.Join(auxDir, envsFile)
	table := texscan.LoadTable[texscan.EnvInfo](path)
	before := maps.Clone(table)
	for name, res := range envprobe.Parse(filepath.Join(auxDir, stem+envprobe.Ext)) {
		info := scanned[name]
		info.Float = res.Float
		scanned[name] = info
	}
	maps.Copy(table, scanned)
	return texscan.SaveTable(path, table, before)
}
