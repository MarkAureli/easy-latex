package cmd

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"

	"github.com/MarkAureli/easy-latex/internal/texscan"
)

const envsFile = "envs.json"

// updateEnvTable refreshes .el/envs.json (environment name -> mandatory
// argument count) from the project's tex files. Entries found in the sources
// overwrite stored ones; environments no longer present are kept.
func updateEnvTable(mainTex string) error {
	path := filepath.Join(auxDir, envsFile)
	table := map[string]int{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &table)
	}
	before := maps.Clone(table)
	maps.Copy(table, texscan.EnvSignatures(texscan.FindTexFiles(mainTex, ".")))
	if maps.Equal(before, table) {
		return nil
	}
	data, err := json.MarshalIndent(table, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
