# internal/pedantic

Configurable pedantic checks run during `el compile`. Violations are errors (non-zero exit). PDF still produced.

## Architecture

Registry-based: each check registers via `init()` → `Register(Check{...})`.

- `PhaseSource` — runs on tex source lines (comment-stripped). Signature: `func(path string, lines []string) []Diagnostic`

## Checks

| Name | Phase | What it flags |
|---|---|---|
| `no-block-citations` | Source | Multi-key cite `\cite{a,b}` or adjacent cites `\cite{a}\cite{b}` |

## Key files

| File | Role |
|---|---|
| `pedantic.go` | `Diagnostic`, `Check`, registry (`Register`, `Get`, `Known`, `AllNames`, `ValidateCheckNames`) |
| `run.go` | `RunSourceChecks`, `readAndStripComments` |
| `block_citations.go` | `no-block-citations` check impl |

## Adding a new check

1. Create `internal/pedantic/<name>.go`
2. Define check func matching `SourceCheckFunc`
3. Call `Register(Check{Name: "...", Phase: ..., Source: ...})` in `init()`
4. No changes needed elsewhere — registry handles discovery
