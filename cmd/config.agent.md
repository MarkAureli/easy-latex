# el config (`config.go`)

Bare `el config` (no subcommand) prints usage error.

## Subcommands

### `el config list [--global]`
Displays all settings in a table: name, effective value, source.
Source is one of `(local)`, `(global)`, `(default)`, or `(ieee default)`.
Outside a project, shows global config only.
`--global` shows only global config (`~/.elconfig.json`); works outside projects.

### `el config set <key> [value] [--global]`
Set a configuration value.
- Bool keys: value is optional (defaults to "true"). Accepts "true" or "false".
- Non-bool keys (e.g. `max-authors`): value is required.
- `--global`: writes to `~/.elconfig.json` instead of `.el/config.json`. Works outside a project.

### `el config unset <key> [--global]`
Unset a configuration value.
- Bool keys: sets to explicit false.
- Non-bool keys: removes the value (restores default).
- `--global`: modifies `~/.elconfig.json`.

## Config keys

Bib options (stored under `bib`):

| Key | Type | Default |
|---|---|---|
| `abbreviate-journals` | bool | true |
| `abbreviate-first-name` | bool | true |
| `brace-titles` | bool | false |
| `ieee-format` | bool | false |
| `max-authors` | int | 0 (unlimited; 5 when ieee-format enabled) |
| `url-from-doi` | bool | false |
| `retry-timeout` | bool | true |

Pedantic checks (stored under `pedantic.checks`, one bool per name):

| Key | Type | Default |
|---|---|---|
| `<check-name>` | bool | false |

Pedantic check keys are generated dynamically from the pedantic registry (`pedantic.AllNames()`). No naming collisions with bib keys.

## Config resolution order

local `.el/config.json` > global `~/.elconfig.json` > built-in default.

## Implementation

`configFields` is built as `bibConfigFields` (static slice) + `pedanticConfigFields()` (one entry per registered check). `findField(key)` does a linear lookup; `loadTargetConfig(cmd)` returns config + save func based on `--global` flag. Shell completion via `configKeyCompletion`.

See `cmd/root.agent.md` for Config / BibConfig / PedanticConfig struct definitions.
