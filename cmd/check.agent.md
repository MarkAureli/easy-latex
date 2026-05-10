# el check (`check.go`)

Run static (source-phase) pedantic checks without compiling.

## Behavior

- Loads merged config; reads `cfg.Pedantic.EnabledNames()`.
- Errors out if no checks are enabled.
- Validates main file exists; resolves all `.tex` files via `texscan.FindTexFiles`.
- Without `--fix`: runs `pedantic.RunSourceChecks` + spell-check; prints two yellow sections — `Pedantics:` then (blank line) `Misspellings:` — to stderr, followed by a yellow summary line `N pedantic(s), M misspelling(s)` (singular when count is 1). Exits 0 unless `--strict` (or config `strict: true`) is set, in which case any violation produces a non-zero exit.
- With `--fix`: captures source-check diagnostics pre-fix, runs `pedantic.RunSourceFixes` (in-place rewrites for fixable checks), re-runs the detector. The pre/post diff produces a `Pedantics (fixed)` section (default colour, header bold), followed by `Pedantics (remaining)` (yellow) for what survived. The summary counts only remaining pedantics; fixed items are informational and never trigger strict-mode failure. Modified file paths are no longer printed (the section entries imply them).

## Scope

- Source-phase only. Dynamic (post-compile) checks are skipped — they require artifacts produced by `el compile`.
- No PDF produced. No bib processing. Pure read (or read+write with `--fix`).

## Flags

| Flag | Effect |
|---|---|
| `--fix` | Apply autofixes from checks that provide a `Fix` (e.g. `single-spaces`). Pure linters (no Fix) are unaffected. |
| `--strict` | Treat any pedantic/spelling violation as a non-zero exit (overrides config). Mutually exclusive with `--no-strict`. |
| `--no-strict` | Force lenient mode (overrides config). Mutually exclusive with `--strict`. |

## Exit codes

- `0` — no violations, or violations exist but strict mode is off (default).
- non-zero — strict mode active (config or `--strict`) and violations remain, or setup error (no checks enabled, missing main, unknown check name).
