package cmd

import (
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/MarkAureli/easy-latex/internal/cmdprobe"
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
// count) after a successful compile, resolving each used or declared command
// from declarations, the compile probe's log (see cmdprobe), the built-in
// table and usage. Scanned entries overwrite stored ones; commands no longer
// present are kept.
func updateCmdTable(stem, mainTex string) error {
	path := filepath.Join(auxDir, cmdsFile)
	table := texscan.LoadTable[int](path)
	before := maps.Clone(table)
	probed := cmdprobe.Parse(filepath.Join(auxDir, stem+cmdprobe.Ext))
	maps.Copy(table, texscan.ScanCmds(projectTexFiles(mainTex), probed))
	return texscan.SaveTable(path, table, before)
}

// writeCmdProbe writes the probe package for every command the project uses
// or declares into the aux dir so the compile can inject it.
func writeCmdProbe(mainTex string) error {
	names := slices.Sorted(maps.Keys(texscan.ScanCmds(projectTexFiles(mainTex), nil)))
	return os.WriteFile(filepath.Join(auxDir, cmdprobe.PkgName+".sty"), cmdprobe.Sty(names), 0644)
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
