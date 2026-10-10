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
| `sentence-terminator` | Source | no | Each document-body prose line must end with `.`/`?`/`!`/`:` or an ellipsis, unless the sentence continues into a list/display-math/theorem block on the next line. Complements `sentence-on-newline`/`block-on-newline`'s one-sentence/block-per-line convention: a line ending is either a sentence end or one of the special cases below. |
| `eqref-tie` | Source | yes | `\eqref` (exact name; `\eqrefs` etc. ignored) in `regText` must be preceded by `~`. Exempt: first content on the line (after leading WS), or glued directly after a byte in `eqrefTieOK` (`~ ( [ { - /`). Math/verbatim skipped. Fix replaces the preceding WS run with `~` (inserts `~` when glued to other chars; `~ \eqref` → `~\eqref`), comment-aware via `texscan.StripComment`; idempotent. |
| `cite-tie` | Source | yes | Same rule as `eqref-tie`, applied to cite commands instead: any match of `texscan.ReCiteCall` (the same command set `no-block-citations` scans) in `regText` must be preceded by `~`, exempt/fix logic identical (reuses `eqrefTieOK`). |
| `env-indent` | Source | yes | Each line indented `(envDepth + braceDepth)*4` spaces. All `\begin{...}`/`\end{...}` and `\[`/`\]` events on a line update the env stack in source order, allowing inline math envs (`\begin{cases}`, `\begin{aligned}`, …) and brace-wrapped end-bodies (`{\end{env}}` from `\newenvironment`). The line's own depth de-dents by the count of consecutive leading-end events at its leading-content position (after WS and any `{` wrappers). Unmatched `{`/`[` carried across lines push brace depth; leading `}`/`]` on a line de-dent that line, then total `{[` vs `}]` advance the running depth. Comments stripped before counting; `\{`/`\}`/`\[`/`\]` ignored (escape-aware). `document` transparent (depth 0 inside). Verbatim envs (`verbatim`, `lstlisting`, `minted`, `comment`, `alltt`, …) preserved untouched and don't update brace depth. Math envs are indented. Fix overwrites leading WS — tabs vanish so order vs `no-tabs` is irrelevant. Comment-only lines re-indented; blank lines untouched. |
| `unused-labels` | ProjectSource | no | `\label{name}` never referenced across the project. Refs scanned: `\ref`/`\Ref`/`\eqref`/`\autoref`/`\Autoref`/`\cref`/`\Cref`/`\crefrange`/`\Crefrange`/`\labelcref`/`\pageref`/`\nameref`/`\vref`/`\Vref`/`\vpageref`/`\autopageref` (curly-brace, comma-list, starred variants) plus `\hyperref[...]`. Verbatim envs skipped; math regions tracked. Hardcoded ignore set silences prophylactic prefixes (see below). |
| `math-bare-word` | Source | no | 2+ consecutive ASCII letters in math mode not inside a text/font wrapper or forming a command. Letters/spaces after a literal `.` are exempt too (tikz `node.anchor` refs, e.g. `($(a1.south west)-(0.1,0.1)$)`). |
| `dashes` | Source | yes | Dash style normalization in text regions. Rules: (1) `–` → `--`; (2) `—` → `---`; (3) `−` → `-` in math, `$-$` in text; (4) `\d\s*-\s*\d` → `\d--\d`; (5) `\d\s*---\s*\d` → `\d--\d`; (6) `----+` → `---`; (7) strip spaces around `---`; (8) `(\w+)\s*--\s*(\w+)` → `w---w` unless either side is a digit-leading word OR both first chars uppercase; (9) `(\w) - (\w)` → `w---w` unless either side digit. Fixpoint loop handles chains. Region mask skips math (except 3-math), verbatim envs, and comments. Brace bodies of class/package/file macros (`\documentclass`, `\usepackage`, `\RequirePackage`, `\LoadClass`, `\WarningFilter`, `\PassOptionsToClass`, `\PassOptionsToPackage`, `\input`, `\include`, `\includeonly`, `\InputIfFileExists`, `\IfFileExists`) are passed through unchanged so package names like `revtex4-2` are preserved. `[...]` option-block content immediately following *any* command (interleaved with `{...}` arg groups, e.g. `\cite[p. 12-14]{key}`, `\begin{tabular}[t]{cc}`, `\sqrt[3]{x}`) is likewise passed through unchanged (`findOptionSkipRanges`). |
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

## sentence-terminator implementation

Convention: a source line ending is either a sentence end (must carry terminal punctuation) or one of a fixed set of special cases (exempt). Per-line, reusing `regionMask` + `blockTokens` from `block-on-newline`, plus two cross-line scans (`nonProseLines`, `braceGroupLines`) and a bounded lookahead.

Exempt (no check performed on the line):
- Blank/comment-only lines
- Outside the document body (`nonProseLines`): before `\begin{document}` / after `\end{document}` when the file has one; the whole file when it has none but contains `\usepackage`/`\RequirePackage`/`\documentclass` (`preambleOnlyTokens`, preamble fragment like `packages.tex`). Other included files are all body.
- Strictly inside a `nonProseEnvs` env: `tabular`/`tabular*`/`tabularx`/`longtable`/`array`, `tikzpicture`, `quantikz`, `algorithm`/`algorithm*`/`algorithmic`, `thebibliography`, `IEEEkeywords`
- Last non-whitespace byte outside `regText` (ends mid-math/mid-verbatim)
- Leading token `\begin`/`\end`, or in `sentenceExemptLeading` (`blockTokens` leading entries **except `\caption`/`\caption*`**, plus `\label`/`\index`)
- Command-only line (`isCommandOnly`): only control sequences + `*`/`[...]`/`{...}` args, first command not in `proseCmds` — e.g. `\hypersetup{..}`, `\def\x{..}`, `\includegraphics[..]{..}`, `\cite[..]{..}`, bare `[1em]`. Covers user macros without listing them.
- `braceGroupLines`: line starts inside a multi-line group owned by a non-prose command (`\author{`, `\IEEEauthorblockA{`, `\tikzset{`, `\newcommand{\x}{`; owner = last control word before `{`, looking past `*`, `[..]`, earlier `{..}` args; anonymous `{` inherits parent; exemption inherits into nested groups), or inside a multi-line `[...]` option group (`\begin{tcolorbox}[`), or the line itself leaves a group open (`…\footnote{`, `\caption{\label{x}`). Groups owned by `proseCmds` (`\caption`, `\footnote`, `\thanks`, `\marginpar`, `\emph`, `\text*`, `\underline`) are checked line by line.
- Ends with `\\` or `\newline` (`hasTrailingLineBreakToken`)
- Next relevant line (`nextLineContinuesSentence`, ≤5 lines, skips blank + command-only lines such as `\vspace`/`\label`/`{\fontsize..}`, and `lookaheadTransparentEnvs` openers: `\begin{widetext}`) opens `\[`, `$$`, a list env, a `displayMathEnvs` env, `*subequations`, or a `theoremEnvNames` env (case-insensitive, star stripped) — the sentence continues through the block, whatever the line ends with.

Terminal-punctuation search (`stripTrailingDecorations`): repeatedly strips trailing decoration commands (`\footnote{...}`, `\label{...}`, `\cite{...}` family, `\index{...}`, `\marginpar{...}`, `\qedhere`, star variants; `trailingDecoratorRe`) *only when non-empty content remains before them* (otherwise the decoration's argument is the line's content and stripping falls through to the generic-closer path, descending into it). Also strips a trailing run of closing brackets/quotes/braces (`)]"'}’”`). Allowed terminators: `.`, `?`, `!`, `:`, `…`, `\ldots`.

Tested on ~20k lines of real papers: residual hits are `;`-split sentences and hard-wrapped prose (true positives), plus rare edge cases (`\flushleft (a) \\ \centering`, lone `=` in figures, `\textbf{b)}` subfigure labels). Not autofixable (Fix is nil).

## no-orphan-line implementation

Uses `el-orphan.sty` (embedded, auto-injected via `\RequirePackage`):
- Start hook installed via `\AddToHook{para/begin}` — kernel hook survives the `\everypar` resets that `\@startsection` / `\@afterheading` perform (a plain `\everypar` append loses our hook after the first sectioning command). Records `S <id> <y> <inputlineno>` (deferred so `\pdflastypos` fires at shipout).
- End hook installed by wrapping `\par` at `\AtBeginDocument`: `\protected\def\par{\el@orig@par\el@orphan@end}`. The kernel `para/end` hook fires too early — `\prevgraf` is still 0 in horizontal mode. Wrapping `\par` places the hook in vertical mode after `\par` finalises, where `\prevgraf` is the just-built paragraph's line count. Records `E <id> <prevgraf>`.
- `\par` is called many times beyond user paragraph ends (section headings, environment exits, repeated `\par`); `\ifel@orphan@inpar` flag ensures exactly one E per matching S.
- Page boundary records via `\AddToHook{shipout/before}`: `\immediate\write` of `P <pageno>` lands in the file before that page's deferred S/E writes.
- `L <id>` written from `para/end` hook (horizontal mode, so the whatsit sits inside the last line and ships on its page). The vertical-mode E whatsit can be carried onto the next page by the page builder (false orphan), so Go uses L's page as end page when present.
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
| `eqref_tie.go` | `eqref-tie` check + fix impl, `findEqrefTieEdits`, `eqrefTieOK` |
| `cite_tie.go` | `cite-tie` check + fix impl, `findCiteTieEdits` (reuses `eqrefTieOK` from `eqref_tie.go`) |
| `sentence_terminator.go` | `sentence-terminator` check impl, `sentenceExemptLeading`, `proseCmds`, `nonProseEnvs`, `nonProseLines`, `braceGroupLines`, `isCommandOnly`, `stripTrailingDecorations`, `nextLineContinuesSentence` |
| `env_indent.go` | `env-indent` check + fix impl, `noIndentBodyEnvs`, `transparentEnvs`, `scanLineEvents` (per-line begin/end token list), `leadingContentPos`, `countBraces`, `nextDecision` state machine (env depth + brace depth) |
| `unused_labels.go` | `unused-labels` check impl, `reLabelCall`/`reRefCall`/`reHyperref`, `ignoredLabelPrefixes` set, project-source phase |
| `math_bare_word.go` | `math-bare-word` check impl, `isArgSkipCmd`, `isASCIILetter` |
| `dashes.go` | `dashes` check + fix impl, `applyDashRules`, `rewriteTextSpan`, `rewriteMathSpan`, `findSkipRanges` (macro-arg skip), `findOptionSkipRanges` (`[...]` command-option skip), fixpoint loop with regex pipeline |
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
