package texscan

import (
	"encoding/json"
	"maps"
	"os"
)

// LoadTable reads a JSON name table (.el/envs.json, .el/cmds.json); a missing
// or malformed file yields an empty table.
func LoadTable[T any](path string) map[string]T {
	table := map[string]T{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &table)
	}
	return table
}

// SaveTable writes the table atomically, skipping the write when it equals
// before.
func SaveTable[T comparable](path string, table, before map[string]T) error {
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

// ApplyEnvDecls refreshes the declaration-derived parts of table: the theorem
// flag of every entry, and the argument counts of declared environments.
// Usage-derived argument counts are left untouched.
func ApplyEnvDecls(table map[string]EnvInfo, args map[string]int, theorems map[string]bool) {
	for name, e := range table {
		e.Theorem = theorems[name]
		table[name] = e
	}
	for name := range theorems {
		e := table[name]
		e.Theorem = true
		table[name] = e
	}
	for name, n := range args {
		e := table[name]
		e.Args = n
		table[name] = e
	}
}
