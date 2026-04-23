# internal/pedantic

Configurable pedantic checks run during `el compile`. Violations are errors (non-zero exit). PDF still produced.

## Architecture

Registry-based: each check registers via `init()` → `Register(Check{...})`. Two phases:

- `PhaseSource` — runs on tex source lines (comment-stripped). Signature: `func(path string, lines []string) []Diagnostic`
- `PhasePostCompile` — runs after compile using synctex data. Signature: `func(stx *SynctexData, sources map[string][]string) []Diagnostic`

## Checks

| Name | Phase | What it flags |
|---|---|---|
| `no-block-citations` | Source | Multi-key cite `\cite{a,b}` or adjacent cites `\cite{a}\cite{b}` |
| `no-math-linebreak` | PostCompile | Inline math (`$...$`, `\(...\)`) spanning multiple PDF lines |

## Key files

| File | Role |
|---|---|
| `pedantic.go` | `Diagnostic`, `Check`, registry (`Register`, `Get`, `Known`, `AllNames`, `ValidateCheckNames`) |
| `run.go` | `RunSourceChecks`, `RunPostCompileChecks`, `readAndStripComments` |
| `synctex.go` | `ParseSynctex` → `SynctexData` (minimal parser: Input lines + `$` records) |
| `block_citations.go` | `no-block-citations` check impl |
| `math_linebreak.go` | `no-math-linebreak` check impl + `hasInlineMath` |

## Synctex format

`$` records mark math-on/math-off in sequential pairs. Format: `$<input>,<line>:<h>,<v>`. Same `v` = same PDF line. Different `v` = line break. Display math (`$$`, `\[...\]`) does NOT produce `$` records.

## Adding a new check

1. Create `internal/pedantic/<name>.go`
2. Define check func matching `SourceCheckFunc` or `PostCompileCheckFunc`
3. Call `Register(Check{Name: "...", Phase: ..., Source/Post: ...})` in `init()`
4. No changes needed elsewhere — registry handles discovery
