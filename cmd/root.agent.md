# cmd/root.go

Config struct + load/save/merge. Shared by all commands that load config.

## Config (`root.go`)

`Config` struct serialised as `.el/config.json` (local) and `~/.elconfig.json` (global):

```
{
  "main": "thesis.tex",
  "bib_files": ["refs.bib"],
  "bib": { ... },
  "pedantic": { "checks": { "no-block-citations": true } }
}
```

| Field | Type | Role |
|---|---|---|
| `main` | string | Main `.tex` file (local only) |
| `bib_files` | []string | Registered `.bib` paths (local only) |
| `bib` | `BibConfig` | Bibliography processing options (see below) |
| `pedantic` | `PedanticConfig` | Per-check enable/disable map |

### `BibConfig`

| Field | Type | Default | Role |
|---|---|---|---|
| `abbreviate_journals` | *bool | true | ISO 4 journal abbrev |
| `brace_titles` | *bool | false | Double-brace title field |
| `ieee_format` | *bool | false | IEEE mode (forces brace-titles, max-authors=5, @misc→@unpublished) |
| `max_authors` | *int | 0 (unlimited) | Truncate authors list; IEEE implies 5 if unset |
| `abbreviate_first_name` | *bool | true | Abbreviate first/middle names to initials |
| `url_from_doi` | *bool | false | Replace url field with `https://doi.org/<doi>` when doi non-empty |
| `retry_timeout` | *bool | true | Re-validate entries that previously timed out |

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
| `loadGlobalConfig()` | Read `~/.elconfig.json` (empty Config if missing) |
| `loadConfig()` | Merged: local > global > default |
| `mergeConfig(local, global)` | Per-field pointer merge for bib + per-key map merge for pedantic checks |
| `saveLocalConfig(cfg)` | Write `.el/config.json` |
| `saveGlobalConfig(cfg)` | Write `~/.elconfig.json` |
| `globalConfigPath()` | Returns path; `globalConfigDir` var overrides home in tests |

JSON `omitzero` on `bib` and `pedantic` fields suppresses empty objects (Go 1.24+).

## PersistentPreRunE

Skips project root check for `init` and all `config` subcommands (config handles project check internally based on `--global` flag). Uses `isConfigCommand(cmd)` helper.
