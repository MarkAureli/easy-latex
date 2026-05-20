# internal/spell

Spell-check engine. Backed by `hunspell` (pipe mode). Surfaces hits as
`spell.Diagnostic` shaped to match `pedantic.Diagnostic`.

## Files

| File | Role |
|---|---|
| `dict.go` | `MergeDicts(out, files...)` — merge layered dict files (global+local × {lang, common}) → hunspell-compatible `.dic` (count header + sorted words). Missing files skipped. Dedup + sort. |
| `ignore.go` | `DefaultIgnoreMacros` (code-baked list of macros whose first arg is skipped during prose extraction). `LoadIgnoreMacros(files...)` returns merged set: defaults + additive entries from files, minus negations (`!macro`). `#` comments and blank lines ignored. |
| `hunspell.go` | `HunspellAvailable(_, warn)` — LookPath check + warn-once. `StartHunspell(lang, personalDict)` — launch `hunspell -a -d <lang> [-p <dict>]`, read banner. `Hunspell.CheckLine(text)` returns `[]Miss`. Banner-read failure → dict missing → caller warns-once. |
| `spell.go` | `Session` (long-lived hunspell handle for LSP — `NewSession`, `Check(files)`, `Stale()`, `Reopen()`, `Close()`). `Stale()` re-stats the six tracked files (layered dicts + ignore lists) and returns true when any mtime changed since session open; `Reopen()` tears the hunspell subprocess down and rebuilds with the same args. `Run(files, lang, auxDir, paths, warn) []Diagnostic` — one-shot wrapper around `Session` for CLI use (el check / el compile). Both: merge dicts → start hunspell → extract prose via `texscan.ProseRuns` per file → check each line → emit `Diagnostic{File, Line, Col, Word, Message, Suggestions}` (`Col` is 1-based byte column on the normalised line). `DefaultPaths(globalDir, auxDir, lang)` builds the conventional set of dict/ignore paths. |
| `manage.go` | File mutation helpers used by `cmd/spell.go`: `ResolveTarget`, `ValidateToken`, `AddTokens`, `RemoveTokens`, `ListTokens`, `CompletionCandidates`. Misspellings cache helpers: `MisspellingsPath`, `WriteMisspellings`, `LoadMisspellings`, `RemoveMisspellings` — drive `el spell add` completion. Sort-and-dedup writes; ignore-mode handles default-vs-user-line semantics (negate-on-remove, drop-negation-on-add). |
| `normalize.go` | `NormalizeAccents(s)` — rewrites every TeX accent macro, alpha-accent (`\v \c \u \H \t`), and zero-arg letter command (`\aa \ae \oe \o \l \i \j \ss \ng \dh \th` + caps) to its precomposed Unicode character (`\ss`→`ß`, `\aa`→`å`, `\'{e}`→`é`, `\v{c}`→`č`, …). All four forms `\X{L}` / `\XL` / `{\X{L}}` / `{\XL}` covered for accents; `{\cmd}` / `{\cmd{}}` / `\cmd<bound>` covered for standalones. Trailing single space after a zero-arg command is eaten (TeX control-word semantics) so `\aa ngstrom`→`ångstrom`. Backed by `internal/texaccent` (shared regex + pipeline). Applied in `Run` before `texscan.ProseRuns`; not length-preserving, so col reports degrade slightly on affected lines. Raw `ß`/`ẞ` and any ss↔ß equivalence are left to the underlying hunspell affix. |

## Dictionary layout

Global: `${XDG_CONFIG_HOME:-~/.config}/easy-latex/spell/{en_GB,en_US,common}.txt`

Local: `<repo>/.el/spell/{en_GB,en_US,common}.txt`

Active lang resolved from `Config.Spelling`. Local wins via existing
`mergeConfig` semantics. Loaded dicts: system hunspell affix/dic for the
active lang + global common + global lang + local common + local lang.

## Ignore lists

Defaults always on. Additive overrides via:
- `${globalDir}/spell/ignore.txt`
- `<repo>/.el/spell/ignore.txt`

One macro name per line. Blank lines and `#` comments ignored. Prefix `!` to
remove a default (e.g. `!cite` re-enables spell-check inside `\cite{…}`
arguments).

## Warn-once

If `hunspell` binary missing or dictionary missing, the package logs a single
warning to the configured writer (default `os.Stderr`) per process and the
check no-ops. Build never fails on tooling absence.

## Hunspell pipe protocol summary

```
^<line>\n         # `^` forces text mode (no token starting with *,+,-,#)
* / + ROOT / -    # correct
& word N off: s1, s2, ...   # miss + suggestions
# word off                  # miss, no suggestions
<blank line>      # end of response for this input line
```

Offsets are 1-based, post-`^`.
