# internal/pedantic

Configurable pedantic checks run during `el compile`. Violations are errors (non-zero exit). PDF still produced.

## Architecture

Registry-based: each check registers via `init()` → `Register(Check{...})`.

- `PhaseSource` — runs on tex source lines (comment-stripped) per file. Diagnostic signature: `func(path string, lines []string) []Diagnostic`. Set `WantRaw: true` on the `Check` to receive raw (non-stripped) lines instead. May optionally provide `Fix` for autofix (raw lines, not comment-stripped): `func(path string, lines []string) ([]string, bool)` — returns rewritten lines and changed flag.
- `PhaseProjectSource` — runs once with all tex files at hand. Signature: `func(files map[string][]string) []Diagnostic`. Read-only (no autofix). Use when a check needs cross-file analysis (e.g. labels defined in one file, referenced in another).
- `PhasePostCompile` — runs after all pdflatex passes complete. Read-only. Signature: `func(auxDir string) []Diagnostic`. No Fix permitted (dynamic checks are non-convergent under autofix). May ship a LaTeX package by setting `Check.StyName` + `Check.Sty` (embedded bytes); `cmd/compile.go` writes each enabled check's sty into `auxDir` and `\RequirePackage`s it before `\input{main.tex}`. Aggregated via `pedantic.PostCompileStys(names)`.

Spelling is **not** part of this registry. It is a parameterised pass keyed by language, driven directly from `cmd/root.go:runSpellCheck` via `internal/spell.Run`. Diagnostics are reshaped to `pedantic.Diagnostic` and merged with pedantic output by callers (`cmd/check.go`, `cmd/compile.go`).

## Checks

| Name | Phase | Fixable | What it flags |
|---|---|---|---|
| `no-block-citations` | Source | no | Multi-key cite `\cite{a,b}` or adjacent cites `\cite{a}\cite{b}` |
| `single-spaces` | Source | yes | Runs of 2+ spaces past leading whitespace. Preserved: leading WS, trailing WS / pre-comment alignment (post-strip), and runs immediately followed by an alignment terminator `=` or `&` (key=value blocks like `\hypersetup`, tabular/align column separators) |
| `no-tabs` | Source | yes | Tab characters outside verbatim regions. Fix expands to spaces with column-aware tabstop (width 4); comments rewritten too |
| `no-trailing-whitespace` | Source | yes | Spaces or tabs at end of any raw line. `WantRaw` so detection sees post-comment WS. Pre-comment alignment WS (`code   % foo`) is preserved (single-spaces' domain); only the tail past the last non-WS byte is stripped. Fixed via `TrimRight(line, " \t")` on raw lines. |
| `block-on-newline` | Source | yes | Block-level token misplaced on its source line. **Leading** tokens (env begin/end, sectioning, `\item`, `\[`/`\]`, page/space breaks, file inclusion, front matter, preamble decls, tabular rules) must start the line. **Trailing** tokens (`\\`, `\newline`) must end the line. Math/verbatim regions skipped. Leading tokens preceded only by `{`/whitespace are allowed (covers `\NewDocumentEnvironment` brace-wrapped bodies). |
| `sentence-on-newline` | Source | yes | Sentence boundary `[.?!] <space> <Capital>` mid-line in text region; abbreviations and digit-only words excluded |
| `env-indent` | Source | yes | Each line indented `(envDepth + braceDepth)*4` spaces. All `\begin{...}`/`\end{...}` and `\[`/`\]` events on a line update the env stack in source order, allowing inline math envs (`\begin{cases}`, `\begin{aligned}`, …) and brace-wrapped end-bodies (`{\end{env}}` from `\newenvironment`). The line's own depth de-dents by the count of consecutive leading-end events at its leading-content position (after WS and any `{` wrappers). Unmatched `{`/`[` carried across lines push brace depth; leading `}`/`]` on a line de-dent that line, then total `{[` vs `}]` advance the running depth. Comments stripped before counting; `\{`/`\}`/`\[`/`\]` ignored (escape-aware). `document` transparent (depth 0 inside). Verbatim envs (`verbatim`, `lstlisting`, `minted`, `comment`, `alltt`, …) preserved untouched and don't update brace depth. Math envs are indented. Fix overwrites leading WS — tabs vanish so order vs `no-tabs` is irrelevant. Comment-only lines re-indented; blank lines untouched. |
| `unused-labels` | ProjectSource | no | `\label{name}` never referenced across the project. Refs scanned: `\ref`/`\Ref`/`\eqref`/`\autoref`/`\Autoref`/`\cref`/`\Cref`/`\crefrange`/`\Crefrange`/`\labelcref`/`\pageref`/`\nameref`/`\vref`/`\Vref`/`\vpageref`/`\autopageref` (curly-brace, comma-list, starred variants) plus `\hyperref[...]`. Verbatim envs skipped; math regions tracked. Hardcoded ignore set silences prophylactic prefixes (see below). |
| `math-bare-word` | Source | no | 2+ consecutive ASCII letters in math mode not inside a text/font wrapper or forming a command |
| `dashes` | Source | yes | Dash style normalization in text regions. Rules: (1) `–` → `--`; (2) `—` → `---`; (3) `−` → `-` in math, `$-$` in text; (4) `\d\s*-\s*\d` → `\d--\d`; (5) `\d\s*---\s*\d` → `\d--\d`; (6) `----+` → `---`; (7) strip spaces around `---`; (8) `(\w+)\s*--\s*(\w+)` → `w---w` unless either side is a digit-leading word OR both first chars uppercase; (9) `(\w) - (\w)` → `w---w` unless either side digit. Fixpoint loop handles chains. Region mask skips math (except 3-math), verbatim envs, and comments. Brace bodies of class/package/file macros (`\documentclass`, `\usepackage`, `\RequirePackage`, `\LoadClass`, `\WarningFilter`, `\PassOptionsToClass`, `\PassOptionsToPackage`, `\input`, `\include`, `\includeonly`, `\InputIfFileExists`, `\IfFileExists`) are passed through unchanged so package names like `revtex4-2` are preserved. |
| `no-math-linebreak` | PostCompile | no | Inline math (`$...$` or `\(...\)`) that spans multiple PDF lines |
| `no-section-linebreak` | PostCompile | no | Sectioning title (`\title`, `\part`, `\chapter`, `\section`, `\subsection`, `\subsubsection`, incl. `*`-variants) that spans multiple PDF lines |
| `no-orphan-line` | PostCompile | no | Orphan paragraphs (2-line paragraph split across pages — line 1 stranded alone at bottom of page A, line 2 on page B). Longer-paragraph orphans not flagged (ambiguous without per-line tracking). |

### `unused-labels` ignore set

Labels whose name (before the first `:`) matches one of these spelled-out prefixes are never flagged, since labeling them by convention is common even when unreferenced:

- Sectioning: `part`, `chapter`, `section`, `subsection`, `subsubsection`, `paragraph`, `subparagraph`, `appendix`
- Theorem-likes: `definition`, `theorem`, `corollary`, `lemma`, `proposition`, `example`, `remark`
- Proof structure: `proof`, `claim`, `conjecture`, `axiom`, `fact`, `observation`, `note`, `assumption`, `hypothesis`, `property`
- Textbook style: `exercise`, `problem`, `solution`, `case`

All other prefixes — including bare labels, `eq:`/`fig:`/`tab:`/etc. abbreviations, and project-defined prefixes — are flagged when unreferenced. Escape hatches: rename to a standard prefix or disable the check.

## no-orphan-line implementation

Uses `el-orphan.sty` (embedded, auto-injected via `\RequirePackage`):
- Start hook installed via `\AddToHook{para/begin}` — kernel hook survives the `\everypar` resets that `\@startsection` / `\@afterheading` perform (a plain `\everypar` append loses our hook after the first sectioning command). Records `S <id> <y> <inputlineno>` (deferred so `\pdflastypos` fires at shipout).
- End hook installed by wrapping `\par` at `\AtBeginDocument`: `\protected\def\par{\el@orig@par\el@orphan@end}`. The kernel `para/end` hook fires too early — `\prevgraf` is still 0 in horizontal mode. Wrapping `\par` places the hook in vertical mode after `\par` finalises, where `\prevgraf` is the just-built paragraph's line count. Records `E <id> <prevgraf>`.
- `\par` is called many times beyond user paragraph ends (section headings, environment exits, repeated `\par`); `\ifel@orphan@inpar` flag ensures exactly one E per matching S.
- Page boundary records via `\AddToHook{shipout/before}`: `\immediate\write` of `P <pageno>` lands in the file before that page's deferred S/E writes.
- Requires LaTeX 2021-06+ (for `\AddToHook`).
- Go check pairs S/E by id, assigns S.page / E.page from the most recent P record seen in file order, flags when `S.page != E.page AND prevgraf == 2`. Longer-paragraph orphans need per-line position data, which pdfTeX doesn't expose; flagged only the unambiguous case.

## no-section-linebreak implementation

Uses `el-sectionpos.sty` (embedded, auto-injected via `\RequirePackage`):
- At `\AtBeginDocument` time, redefines `\part`/`\chapter`/`\section`/`\subsection`/`\subsubsection` (via xparse `s o m` so star + optional short + mandatory all flow through). Each redefined command:
  1. Increments `\el@secid`, writes `M <id> <inputlineno> <kind>` immediately (call-site line).
  2. Wraps the mandatory title arg in `\el@secstart{kind}{id} ... \el@secend{kind}{id}` via `\edef` (captures id at call site so concurrent typesetting is safe).
  3. Forwards to original `\el@orig@<name>` with star / optional preserved.
- `\el@secstart`/`\el@secend` use `\pdfsavepos` + deferred `\write` to record `S <id> <y-sp> <kind>` / `E <id> <y-sp> <kind>` when the title actually typesets.
- `\title` is special: by `\AtBeginDocument` the user has already called it and the body lives in `\@title`. Wrap `\@title` in-place with S/E markers; M record carries line=0. Go checker recovers line by grepping main.tex for `\title{` (`reTitleCall`).
- hyperref compatibility: when `hyperref` is loaded, `\pdfstringdefDisableCommands` makes `\el@secstart` / `\el@secend` expand to nothing during PDF-string conversion (bookmarks, `/Title` metadata). Without this hook hyperref emits "Token not allowed in a PDF string (Unicode)" for every wrapped heading.
- Writes `<jobname>.sectionpos`. Go check pairs S/E by id; if y-positions differ → diagnostic on M's line.

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
| `pedantic.go` | `Diagnostic`, `Check`, `SourceCheckFunc`, `SourceFixFunc`, `ProjectSourceCheckFunc`, `PostCompileCheckFunc`, registry (`Register`, `Get`, `Known`, `AllNames`, `ValidateCheckNames`) |
| `run.go` | `RunSourceChecks` (per-file + project-source dispatch; passes raw or stripped lines per check's `WantRaw`), `RunPostCompileChecks`, `RunSourceFixes`, `HasPostCompileChecks`, `HasFixableChecks`, `readSource` (returns raw + stripped) |
| `region.go` | `regionMask` — per-byte text/math/verbatim classification; tracks `$`, `\(\)`, `\[\]`, math envs, verbatim envs across lines |
| `block_citations.go` | `no-block-citations` check impl |
| `single_spaces.go` | `single-spaces` check + fix impl |
| `trailing_whitespace.go` | `no-trailing-whitespace` check + fix impl, `trimRightWS` |
| `no_tabs.go` | `no-tabs` check + fix impl, column-aware tab expansion (`tabWidth=4`) |
| `block_on_newline.go` | `block-on-newline` check + fix impl, `blockTokens` (leading/trailing kinds), `nextTokenAt` parser |
| `sentence_on_newline.go` | `sentence-on-newline` check + fix impl, `sentenceAbbrevs` set |
| `env_indent.go` | `env-indent` check + fix impl, `noIndentBodyEnvs`, `transparentEnvs`, `scanLineEvents` (per-line begin/end token list), `leadingContentPos`, `countBraces`, `nextDecision` state machine (env depth + brace depth) |
| `unused_labels.go` | `unused-labels` check impl, `reLabelCall`/`reRefCall`/`reHyperref`, `ignoredLabelPrefixes` set, project-source phase |
| `math_bare_word.go` | `math-bare-word` check impl, `isTextMathCmd`, `isASCIILetter` |
| `dashes.go` | `dashes` check + fix impl, `applyDashRules`, `rewriteTextSpan`, `rewriteMathSpan`, fixpoint loop with regex pipeline |
| `math_linebreak.go` | `no-math-linebreak` check impl, `parseMathPos`, `MathPosSty` embed |
| `el-mathpos.sty` | LaTeX package for inline-math position tracking (embedded into binary) |
| `section_linebreak.go` | `no-section-linebreak` check impl, `parseSectionPos`, `SectionPosSty` embed, `findTitleLine` |
| `el-sectionpos.sty` | LaTeX package for sectioning-title position tracking (embedded into binary) |
| `orphan.go` | `no-orphan-line` check impl, `parseOrphan`, `OrphanSty` embed |
| `el-orphan.sty` | LaTeX package for paragraph/page-boundary tracking (embedded into binary) |

## Adding a new check

1. Create `internal/pedantic/<name>.go`
2. Define detector matching `SourceCheckFunc` (source phase) or `PostCompileCheckFunc` (dynamic phase)
3. Optionally for source-phase checks: define a `SourceFixFunc` that rewrites raw lines
4. Call `Register(Check{Name: "...", Phase: ..., Source: ..., Fix: ..., PostCompile: ...})` in `init()`
5. No changes needed elsewhere — registry handles discovery; config CLI auto-exposes the name
