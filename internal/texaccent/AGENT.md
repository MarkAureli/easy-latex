# internal/texaccent

Shared TeX accent / standalone-letter macro recogniser. Provides regexes and
a small pipeline; consumers pick their own output flavour via callbacks.

Currently used by:

- `internal/bib/key.go` — `latexToASCII` (ASCII for cite-key generation)
- `internal/spell/normalize.go` — `NormalizeAccents` (precomposed Unicode for hunspell)

## Exports

| Symbol | Role |
|---|---|
| `NonAlphaAccent` (regex) | All four forms `{\X{L}}` / `{\XL}` / `\X{L}` / `\XL` for X ∈ `' " ` ^ ~ . =` |
| `AlphaAccent` (regex) | Braced forms `{\X{L}}` / `\X{L}` for X ∈ `u v H c t`. Unbraced excluded to avoid colliding with `\url`, `\vspace`, `\Huge`, `\centering`, `\textbf` |
| `Standalone` (regex) | Zero-arg letter commands `\ss \SS \aa \AA \ae \AE \oe \OE \o \O \l \L \i \j \ng \dh \DH \th \TH` followed by a boundary byte (space/tab → control-word terminator, non-letter, or end-of-string) |
| `AccentFn` | `func(cmd, letter string) (string, bool)` — return ok=false to leave the macro untouched |
| `StandaloneFn` | `func(cmd, boundary string) (string, bool)` — callback decides whether to keep the boundary (e.g. preserve `,`) or eat it (e.g. control-word space) |
| `ApplyNonAlphaAccent`, `ApplyAlphaAccent`, `ApplyStandalone` | Single-pass rewriters |
| `Expand` | Convenience: non-alpha → standalone → alpha order |

## Forms covered

```
\'e        \'{e}        {\'e}        {\'{e}}
\"a        \"{a}        {\"a}        {\"{a}}
\v{c}      {\v{c}}                                  (alpha-accent, braced-only)
\ss        \ss.         \ss<space>letter            (standalone w/ boundary)
\aa        \AE!         \oe<space>letter
```

## Notes

- Alternation in `NonAlphaAccent` and `AlphaAccent` is longest-form-first so
  outer-brace wrappers win under Go's leftmost-first regex engine.
- `Standalone` does NOT match `{\ss}` or `{\ss{}}` group forms — spell's
  `NormalizeSharpS` already collapses those before `texaccent` runs.
- Replacements are NOT length-preserving. Callers that rely on column offsets
  (LSP positions) inherit a small drift on lines containing accents.
