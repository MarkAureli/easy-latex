# el config (`config.go`)

Bare `el config` (no subcommand) prints usage error.

## Subcommands

### `el config list [--global]`
Displays all settings in a table: name, effective value, source.
Source is one of `(local)`, `(global)`, `(default)`, or `(ieee default)`.
Outside a project, shows global config only.
`--global` shows only global config (`~/.config/easy-latex/config.json`); works outside projects.

### `el config set <key> [value] [--global]`
Set a configuration value.
- Bool keys: value is optional (defaults to "true"). Accepts "true" or "false".
- Non-bool keys (e.g. `max-authors`): value is required.
- `--global`: writes to `~/.config/easy-latex/config.json` instead of `.el/config.json`. Works outside a project.

### `el config unset <key> [--global]`
Unset a configuration value.
- Bool keys: sets to explicit false.
- Non-bool keys: removes the value (restores default).
- `--global`: modifies `~/.config/easy-latex/config.json`.

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

Spell-check (stored at top level, not under `pedantic.checks`):

| Key | Type | Allowed values | Default |
|---|---|---|---|
| `spelling` | string | `en_GB`, `en_US` | unset (off) |

`el config set spelling en_GB` enables British-English spell-check; `el config unset spelling` turns it off. Validation rejects any other value. The `spelling` pedantic check is automatically appended to the enabled list when this key is set (see `effectiveEnabledChecks` in `root.go`).

### `pedantic` (alias)

`el config set pedantic [value]` and `el config unset pedantic` apply the
operation to every registered pedantic check at once. Equivalent to running
`el config set <check> [value]` (or `unset`) for each `<check>` in
`pedantic.AllNames()`. The alias is not itself a `configField`: it has no
display row in `el config list` and is not persisted under its own name —
only the underlying per-check entries in `pedantic.checks` are written.
Recognised by string match against `pedanticAliasKey` in `runConfigSet` /
`runConfigUnset` before the normal `findField` lookup.

## Config resolution order

local `.el/config.json` > global `~/.config/easy-latex/config.json` > built-in default.

## Implementation

`configFields` is built as `bibConfigFields` (static slice) + `pedanticConfigFields()` (one entry per registered check). `findField(key)` does a linear lookup; `loadTargetConfig(cmd)` returns config + save func based on `--global` flag. Shell completion via `configKeyCompletion`.

See `cmd/root.agent.md` for Config / BibConfig / PedanticConfig struct definitions.
