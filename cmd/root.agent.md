# cmd/root.go

Config struct + load/save. Shared by all commands that load config.

## Config (`root.go`)

`Config` struct serialised as `.el/config.json`:

| Field | Type | Default | Role |
|---|---|---|---|
| `main` | string | — | Main `.tex` file |
| `bib_files` | []string | — | Registered `.bib` paths |
| `abbreviate_journals` | *bool | true | ISO 4 journal abbrev |
| `brace_titles` | *bool | false | Double-brace title field |
| `ieee_format` | *bool | false | IEEE mode (forces brace-titles, max-authors=5, @misc→@unpublished) |
| `max_authors` | *int | 0 (unlimited) | Truncate authors list; IEEE implies 5 if unset |
| `abbreviate_first_name` | *bool | true | Abbreviate first/middle names to initials |
| `url_from_doi` | *bool | false | Replace url field with `https://doi.org/<doi>` when doi non-empty |

Nil pointer = use default. Accessor methods on `*Config` encode defaults.
