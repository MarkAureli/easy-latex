# cmd/root.go

Config struct + load/save/merge. Shared by all commands that load config.

## Config (`root.go`)

`Config` struct serialised as `.el/config.json` (local) and `~/.elconfig.json` (global):

| Field | Type | Default | Role |
|---|---|---|---|
| `main` | string | — | Main `.tex` file (local only) |
| `bib_files` | []string | — | Registered `.bib` paths (local only) |
| `abbreviate_journals` | *bool | true | ISO 4 journal abbrev |
| `brace_titles` | *bool | false | Double-brace title field |
| `ieee_format` | *bool | false | IEEE mode (forces brace-titles, max-authors=5, @misc→@unpublished) |
| `max_authors` | *int | 0 (unlimited) | Truncate authors list; IEEE implies 5 if unset |
| `abbreviate_first_name` | *bool | true | Abbreviate first/middle names to initials |
| `url_from_doi` | *bool | false | Replace url field with `https://doi.org/<doi>` when doi non-empty |
| `retry_timeout` | *bool | true | Re-validate entries that previously timed out during validation |
| `pedantic` | []string | nil | Enabled pedantic check names (e.g. `no-block-citations`, `no-math-linebreak`) |

Nil pointer = use default. Accessor methods on `*Config` encode defaults.

## Config loading

| Function | Description |
|---|---|
| `loadLocalConfig()` | Read `.el/config.json` |
| `loadGlobalConfig()` | Read `~/.elconfig.json` (empty Config if missing) |
| `loadConfig()` | Merged: local > global > default |
| `mergeConfig(local, global)` | Per-field merge; local pointer wins if non-nil |
| `saveLocalConfig(cfg)` | Write `.el/config.json` |
| `saveGlobalConfig(cfg)` | Write `~/.elconfig.json` |
| `globalConfigPath()` | Returns path; `globalConfigDir` var overrides home in tests |

## PersistentPreRunE

Skips project root check for `init` and all `config` subcommands (config handles project check internally based on `--global` flag). Uses `isConfigCommand(cmd)` helper.
