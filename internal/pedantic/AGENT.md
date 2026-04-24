# internal/pedantic

Configurable pedantic checks run during `el compile`. Violations are errors (non-zero exit). PDF still produced.

## Architecture

Registry-based: each check registers via `init()` → `Register(Check{...})`.

- `PhaseSource` — runs on tex source lines (comment-stripped). Signature: `func(path string, lines []string) []Diagnostic`
- `PhasePostCompile` — runs after all pdflatex passes complete. Signature: `func(auxDir string) []Diagnostic`

## Checks

| Name | Phase | What it flags |
|---|---|---|
| `no-block-citations` | Source | Multi-key cite `\cite{a,b}` or adjacent cites `\cite{a}\cite{b}` |
| `no-math-linebreak` | PostCompile | Inline math (`$...$` or `\(...\)`) that spans multiple PDF lines |

## no-math-linebreak implementation

Uses `el-mathpos.sty` (embedded, auto-injected via `\RequirePackage` when enabled):
- Hooks into LaTeX's `\everymath` token register (appended, not replaced)
- `\pdfsavepos` + deferred `\write` record start/end y-positions per math expression
- Local `\ifelmath@outer` flag prevents tracking nested math (sub/superscripts)
- Writes `S <id> <y-sp> <line>` / `E <id> <y-sp> <line>` to `<jobname>.mathpos`

Post-compile: `checkMathLinebreak` parses `.mathpos`, flags pairs with differing y-coords. Validates source line contains `$` or `\(` to filter false positives from `\maketitle`, bibliography, etc.

Injection: `compile.go` writes sty to `.el/`, sets `TEXINPUTS` to include aux dir, wraps input as `\RequirePackage{el-mathpos}\input{main.tex}` with `-jobname=<stem>`.

## Key files

| File | Role |
|---|---|
| `pedantic.go` | `Diagnostic`, `Check`, registry (`Register`, `Get`, `Known`, `AllNames`, `ValidateCheckNames`) |
| `run.go` | `RunSourceChecks`, `RunPostCompileChecks`, `HasPostCompileChecks`, `readAndStripComments` |
| `block_citations.go` | `no-block-citations` check impl |
| `math_linebreak.go` | `no-math-linebreak` check impl, `parseMathPos`, `MathPosSty` embed |
| `el-mathpos.sty` | LaTeX package for position tracking (embedded into binary) |

## Adding a new check

1. Create `internal/pedantic/<name>.go`
2. Define check func matching `SourceCheckFunc` or `PostCompileCheckFunc`
3. Call `Register(Check{Name: "...", Phase: ..., Source/PostCompile: ...})` in `init()`
4. No changes needed elsewhere — registry handles discovery
