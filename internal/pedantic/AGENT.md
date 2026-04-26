# internal/pedantic

Configurable pedantic checks run during `el compile`. Violations are errors (non-zero exit). PDF still produced.

## Architecture

Registry-based: each check registers via `init()` → `Register(Check{...})`.

- `PhaseSource` — runs on tex source lines (comment-stripped). Diagnostic signature: `func(path string, lines []string) []Diagnostic`. May optionally provide `Fix` for autofix (raw lines, not comment-stripped): `func(path string, lines []string) ([]string, bool)` — returns rewritten lines and changed flag.
- `PhasePostCompile` — runs after all pdflatex passes complete. Read-only. Signature: `func(auxDir string) []Diagnostic`. No Fix permitted (dynamic checks are non-convergent under autofix).

## Checks

| Name | Phase | Fixable | What it flags |
|---|---|---|---|
| `no-block-citations` | Source | no | Multi-key cite `\cite{a,b}` or adjacent cites `\cite{a}\cite{b}` |
| `single-spaces` | Source | yes | Runs of 2+ spaces past leading whitespace; comment tail preserved |
| `block-on-newline` | Source | yes | Block-level token misplaced on its source line. **Leading** tokens (env begin/end, sectioning, `\item`, `\[`/`\]`, page/space breaks, file inclusion, front matter, preamble decls, tabular rules) must start the line. **Trailing** tokens (`\\`, `\newline`) must end the line. Math/verbatim regions skipped. Leading tokens preceded only by `{`/whitespace are allowed (covers `\NewDocumentEnvironment` brace-wrapped bodies). |
| `sentence-on-newline` | Source | yes | Sentence boundary `[.?!] <space> <Capital>` mid-line in text region; abbreviations and digit-only words excluded |
| `no-math-linebreak` | PostCompile | no | Inline math (`$...$` or `\(...\)`) that spans multiple PDF lines |

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
| `pedantic.go` | `Diagnostic`, `Check`, `SourceCheckFunc`, `SourceFixFunc`, `PostCompileCheckFunc`, registry (`Register`, `Get`, `Known`, `AllNames`, `ValidateCheckNames`) |
| `run.go` | `RunSourceChecks`, `RunPostCompileChecks`, `RunSourceFixes`, `HasPostCompileChecks`, `HasFixableChecks`, `readAndStripComments` |
| `region.go` | `regionMask` — per-byte text/math/verbatim classification; tracks `$`, `\(\)`, `\[\]`, math envs, verbatim envs across lines |
| `block_citations.go` | `no-block-citations` check impl |
| `single_spaces.go` | `single-spaces` check + fix impl |
| `block_on_newline.go` | `block-on-newline` check + fix impl, `blockTokens` (leading/trailing kinds), `nextTokenAt` parser |
| `sentence_on_newline.go` | `sentence-on-newline` check + fix impl, `sentenceAbbrevs` set |
| `math_linebreak.go` | `no-math-linebreak` check impl, `parseMathPos`, `MathPosSty` embed |
| `el-mathpos.sty` | LaTeX package for position tracking (embedded into binary) |

## Adding a new check

1. Create `internal/pedantic/<name>.go`
2. Define detector matching `SourceCheckFunc` (source phase) or `PostCompileCheckFunc` (dynamic phase)
3. Optionally for source-phase checks: define a `SourceFixFunc` that rewrites raw lines
4. Call `Register(Check{Name: "...", Phase: ..., Source: ..., Fix: ..., PostCompile: ...})` in `init()`
5. No changes needed elsewhere — registry handles discovery; config CLI auto-exposes the name
