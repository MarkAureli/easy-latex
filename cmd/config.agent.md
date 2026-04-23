# el config (`config.go`)

## Display mode (no flags)

`el config` with no flags displays all settings in a table: name, effective value, source (`(default)`, `(set)`, or `(ieee default)`). Uses `text/tabwriter`. `displayConfig(cfg *Config)` handles rendering.

## Setter mode (flags)

Flags (all optional):

| Flag | Type | Default |
|---|---|---|
| `--abbreviate-journals` | bool | true |
| `--brace-titles` | bool | false |
| `--ieee-format` | bool | false |
| `--max-authors` | int | 0 |
| `--abbreviate-first-name` | bool | true |
| `--url-from-doi` | bool | false |
| `--retry-timeout` | bool | true |

Loads `.el/config.json`, sets only changed flags (via `cmd.Flags().Changed`), saves back.

See `cmd/root.agent.md` for Config struct definition.
