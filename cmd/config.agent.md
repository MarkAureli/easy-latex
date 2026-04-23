# el config (`config.go`)

## Display mode

`el config --list` displays all settings in a table: name, effective value, source.
Source is one of `(local)`, `(global)`, `(default)`, or `(ieee default)`.
Requires being inside a project (`.el/` directory).

`el config --list --global` shows global config only. Works outside projects.

Bare `el config` (no flags/subcommand) prints usage error.

## Subcommands

### `el config set <key> [value] [--global]`
Set a configuration value.
- Bool keys: value is optional (defaults to "true"). Accepts "true" or "false".
- Non-bool keys (e.g. max-authors): value is required.
- `--global`: writes to `~/.elconfig.json` instead of `.el/config.json`. Works outside a project.

### `el config unset <key> [--global]`
Unset a configuration value.
- Bool keys: sets the value to false.
- Non-bool keys: removes the value from config (restores default).
- `--global`: modifies `~/.elconfig.json`.

## Config keys

| Key | Type | Default |
|---|---|---|
| `abbreviate-journals` | bool | true |
| `abbreviate-first-name` | bool | true |
| `brace-titles` | bool | false |
| `ieee-format` | bool | false |
| `max-authors` | int | 0 (unlimited; 5 when ieee-format enabled) |
| `url-from-doi` | bool | false |
| `retry-timeout` | bool | true |

## Config resolution order

local `.el/config.json` > global `~/.elconfig.json` > built-in default.

## Implementation

`configField` registry in `configFields` slice maps key names to struct field accessors.
`loadTargetConfig(cmd)` returns config + save func based on `--global` flag.
Shell completion via `configKeyCompletion`.

See `cmd/root.agent.md` for Config struct definition.
