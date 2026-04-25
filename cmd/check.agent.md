# el check (`check.go`)

Run static (source-phase) pedantic checks without compiling.

## Behavior

- Loads merged config; reads `cfg.Pedantic.EnabledNames()`.
- Errors out if no checks are enabled.
- Validates main file exists; resolves all `.tex` files via `texscan.FindTexFiles`.
- Without `--fix`: runs `pedantic.RunSourceChecks`, prints diagnostics to stderr, exits non-zero on any violation.
- With `--fix`: first runs `pedantic.RunSourceFixes` (in-place rewrites for fixable checks), then runs detector on post-fix content. Reports modified file paths to stdout.

## Scope

- Source-phase only. Dynamic (post-compile) checks are skipped — they require artifacts produced by `el compile`.
- No PDF produced. No bib processing. Pure read (or read+write with `--fix`).

## Flags

| Flag | Effect |
|---|---|
| `--fix` | Apply autofixes from checks that provide a `Fix` (e.g. `single-spaces`). Pure linters (no Fix) are unaffected. |

## Exit codes

- `0` — no violations after run (and after fix if `--fix`).
- non-zero — violations remain, or setup error (no checks enabled, missing main, unknown check name).
