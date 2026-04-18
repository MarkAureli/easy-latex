# el config (`config.go`)

Flags (all optional; error if none given):

| Flag | Type | Default |
|---|---|---|
| `--abbreviate-journals` | bool | true |
| `--brace-titles` | bool | false |
| `--ieee-format` | bool | false |
| `--max-authors` | int | 0 |
| `--abbreviate-first-name` | bool | true |
| `--url-from-doi` | bool | false |

Loads `.el/config.json`, sets only changed flags (via `cmd.Flags().Changed`), saves back.

See `cmd/root.agent.md` for Config struct definition.
