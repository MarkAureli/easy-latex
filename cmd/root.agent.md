# cmd/root.go

Config struct + load/save/merge. Shared by all commands that load config.

## Config (`root.go`)

`Config` struct serialised as `.el/config.json` (local) and `${XDG_CONFIG_HOME:-~/.config}/easy-latex/config.json` (global):

```
{
  "main": "thesis.tex",
  "bib_files": ["refs.bib"],
  "bib": { ... },
  "pedantic": { "checks": { "no-block-citations": true } },
  "spelling": "en_US",
  "strict": false,
  "engine": "xelatex"
}
```

| Field | Type | Role |
|---|---|---|
| `main` | string | Main `.tex` file (local only) |
| `bib_files` | []string | Registered `.bib` paths (local only) |
| `bib` | `BibConfig` | Bibliography processing options (see below) |
| `pedantic` | `PedanticConfig` | Per-check enable/disable map |
| `spelling` | *string | Spell-check language: `en_GB`, `en_US`, or unset (off). Triggers `runSpellCheck` in `cmd/check.go` and `cmd/compile.go`; independent of the pedantic registry. |
| `strict` | *bool | When true, `el check` and `el compile` exit non-zero on any pedantic / spelling / compile-time warning. Default false. CLI flags `--strict` / `--no-strict` override. Helper: `cfg.strict()`. |
| `engine` | *string | LaTeX engine: `pdflatex` (default), `xelatex`, `lualatex`. Validated via `validateEngine`. Helper: `cfg.engine()`. Resolved by `resolveEngine` in `cmd/compile.go` with precedence `--engine` flag → magic comment (`% !TEX program = ...`) → config → default. |

### `BibConfig`

| Field | Type | Default | Role |
|---|---|---|---|
| `abbreviate_journals` | *bool | true | ISO 4 journal abbrev |
| `brace_titles` | *bool | false | Double-brace title field |
| `max_authors` | *int | 0 (unlimited) | Truncate authors list |
| `abbreviate_first_name` | *bool | true | Abbreviate first/middle names to initials |
| `url_from_doi` | *bool | false | Replace url field with `https://doi.org/<doi>` when doi non-empty |
| `retry_timeout` | *bool | true | Re-validate entries that previously timed out |
| `arxiv_as_unpublished` | *bool | false | Convert arXiv @misc entries to @unpublished |

**IEEEtran auto-detection** — `el init` detects `\documentclass{IEEEtran}` and writes brace-titles=true, max-authors=5, arxiv-as-unpublished=true into the local `.el/config.json`.

### `PedanticConfig`

`Checks map[string]*bool` — name → state. `nil` = inherit, `*true` = enabled, `*false` = disabled.

Helpers:
- `Enabled(name)` — true iff explicitly set to true
- `EnabledNames()` — sorted list of enabled check names

Accessor methods on `*Config` (e.g. `cfg.ieeeFormat()`, `cfg.maxAuthors()`) encode defaults and delegate to `cfg.Bib.X`.

## Config loading

| Function | Description |
|---|---|
| `loadLocalConfig()` | Read `.el/config.json` |
| `loadGlobalConfig()` | Read `${XDG_CONFIG_HOME:-~/.config}/easy-latex/config.json` (empty Config if missing) |
| `loadConfig()` | Merged: local > global > default |
| `mergeConfig(local, global)` | Per-field pointer merge for bib + spelling + per-key map merge for pedantic checks |
| `saveLocalConfig(cfg)` | Write `.el/config.json` |
| `saveGlobalConfig(cfg)` | Write global config; auto-creates parent dir |
| `GlobalConfigDir()` | Returns the global dir (honours `globalConfigDir` test override, then `XDG_CONFIG_HOME`, then `~/.config`). Used by `internal/spell` to locate `spell/{lang,common,ignore}.txt`. |
| `globalConfigPath()` | `GlobalConfigDir()/config.json` |
| `runSpellCheck(cfg, texFiles)` | Runs `internal/spell.Run` if `cfg.Spelling != nil`, returning `[]pedantic.Diagnostic`. Reads tex files, strips comments via `texscan.StripComment`. Callers display spelling diagnostics in their own `Spelling:` section (do not merge into pedantics). |
| `sortDiagnostics(d)` | In-place stable sort of `[]pedantic.Diagnostic` by File then Line. |
| `resolveStrict(cfg, strictFlag, noStrictFlag)` | Strict-mode resolution: `--strict` wins, then `--no-strict`, then `cfg.strict()`. |
| `printDiagSection(w, label, diags, headerColor, entryColor, colors)` | Prints a bold `<label>:` header in `headerColor` followed by indented diag lines in `entryColor`. Pass `""` for default-colour rendering (used by the `Pedantics (fixed)` section). Caller is responsible for blank-line separation between adjacent sections. |
| `diffDiagnostics(before, after)` | Set difference: returns diagnostics present in `before` but not in `after` (matched by File+Line+Message). Used by `el check`/`el compile` with `--fix` to derive the "fixed" subset from pre/post detector runs. |
| `printSummary(w, ped, spell, warn, includeWarnings, colors)` | Yellow/bold one-line summary preceded by a blank line. No-op if all counts zero. `includeWarnings=false` for `el check` (omits compile-warning count). |

JSON `omitzero` on `bib` and `pedantic` fields suppresses empty objects (Go 1.24+).

## PersistentPreRunE

Skips project root check for:
- `init`, `help`, `completion`, `lsp`
- all `config` subcommands (`isConfigCommand`) — config handles project check internally based on `--global` flag
- all `spell` subcommands (`isSpellCommand`) — same pattern, project check deferred to subcommand
- all `bib` subcommands except `bib parse` (`isBibCommand`) — the bib cache is global, so `add`/`list`/`remove` work anywhere; only `parse` needs a project to find registered `.bib` files
