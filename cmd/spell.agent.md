# el spell (`spell.go`)

Manage spell-check dictionaries and the macro-arg ignore list. Subcommands
write/read flat token files; `cmd/root.go:runSpellCheck` consumes them at
compile/check time via `internal/spell`.

## Subcommands

```
el spell add    <token>... [--global] [--ignore | --common]
el spell remove <token>... [--global] [--ignore | --common]
el spell list             [--global] [--ignore | --common]
```

`--ignore` and `--common` are mutually exclusive (validated up front).

## Target resolution

| flags | target |
|---|---|
| (none) | `<repo>/.el/spell/<lang>.txt` |
| `--global` | `${globalDir}/spell/<lang>.txt` |
| `--common` | `<repo>/.el/spell/common.txt` |
| `--global --common` | `${globalDir}/spell/common.txt` |
| `--ignore` | `<repo>/.el/spell/ignore.txt` |
| `--global --ignore` | `${globalDir}/spell/ignore.txt` |

`<lang>` resolved from merged `Config.Spelling`; the per-lang targets error if
unset.

## Semantics

- **Validation** — `ValidateToken` rejects empty/whitespace tokens.
- **Sort + dedup** — every write rewrites the target as sorted unique lines.
  User comments (`#`) and blank lines are NOT preserved.
- **Add to dict** — appends new tokens; duplicates are no-ops.
- **Remove from dict** — drops matching lines; missing tokens are no-ops.
- **Add to ignore** — for a token already covered by `DefaultIgnoreMacros` (or
  already present in the file) → no-op. For a token whose negation `!token` is
  in the file → the negation is dropped (un-negate). Otherwise → append.
- **Remove from ignore** — drops a matching user-added line if present, AND
  writes `!token` to negate any matching default. Both may apply.
- Writes auto-create parent directories.

## Flow

1. `resolveSpellTarget(cmd)` — flag-mutex check, `findProjectRoot()` + `chdir`
   for non-global, `loadConfig()` for lang resolution, then
   `spell.ResolveTarget(...)`.
2. Verb runner → `spell.AddTokens` / `spell.RemoveTokens` / `spell.ListTokens`.
3. Print summary (added/removed counts) or token list.

## Auto-completion

`spellRemoveCompletion` uses `spell.CompletionCandidates(path, isIgnore)`:
file lines (with `!` prefix stripped) ∪ `DefaultIgnoreMacros` (when `isIgnore`).
Tokens already in `args` are filtered out so a second TAB doesn't repeat.

`spellAddCompletion` reads the per-project misspellings cache via
`spell.LoadMisspellings(spell.MisspellingsPath(<root>/.el))`. Returns no
candidates when `--ignore` is set or when no project root is found. Tokens
already in `args` are filtered. `runSpellAdd` calls `spell.RemoveMisspellings`
for the added tokens after a successful add (non-`--ignore` only), so the next
TAB no longer offers them.

## Misspellings cache

`runSpellCheck` (root.go) writes the unique misspelled words observed in the
latest run to `<auxDir>/spell/misspellings.txt` (sorted-unique). The file is
overwritten on every `el check` / `el compile` pass. Helpers live in
`internal/spell/manage.go`: `MisspellingsPath`, `WriteMisspellings`,
`LoadMisspellings`, `RemoveMisspellings`. `spell.Diagnostic` carries the raw
`Word` so writers don't have to re-parse the message.

## PreRunE bypass

`isSpellCommand(cmd)` returns true for `spellCmd` and any descendant; root's
`PersistentPreRunE` skips its `findProjectRoot()` step so `--global` invocations
work outside a project. Subcommands resolve the project root themselves only
when needed.
