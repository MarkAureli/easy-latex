# easy-latex

A minimal CLI tool for compiling LaTeX documents without the noise.

## Commands

### `el init`

Run in a folder containing a `.tex` file with `\begin{document}`. Detects the main file, creates an `.aux_dir/` for auxiliary files, and saves the configuration to `.el.json`.

```
$ el init
Initialized. Main file: thesis.tex
```

If multiple eligible `.tex` files are found, you will be prompted to pick one.

### `el compile`

Compiles the document using `pdflatex`. Only warnings and errors are printed — all other LaTeX output is suppressed. On success, a symlink to the PDF is created in the project root.

```
$ el compile
Compiled successfully -> thesis.pdf
```

If the document uses a bibliography, `el` automatically detects the required tool and runs the full compilation sequence:

- `\bibliography{}` (natbib, plain bibtex) → `pdflatex` → `bibtex` → `pdflatex`
- `\usepackage{biblatex}` → `pdflatex` → `biber` → `pdflatex`

Detection is based on the auxiliary files produced by the first `pdflatex` pass (`.bcf` for biber, `\bibdata` in `.aux` for bibtex), so it works regardless of whether the bibliography is defined in a separate `.bib` file or embedded in the document.

Use `-o` / `--open` to open the PDF immediately after compilation:

```
$ el compile -o
```

## Installation

Requires Go 1.21+ and a working TeX Live / MacTeX installation.

```bash
git clone git@github.com:MarkAureli/easy-latex.git
cd easy-latex
make install
```

This places the `el` binary in `~/go/bin`. Make sure that directory is in your `$PATH`:

```zsh
# Add to ~/.zshrc
export PATH="$HOME/go/bin:$PATH"
```

### TeX tools not in PATH?

On macOS, TeX Live installs its binaries to `/Library/TeX/texbin/`. `el` checks this location automatically as a fallback for `pdflatex`, `bibtex`, and `biber`, so it works even if you haven't added it to your `$PATH`.

## What `el` adds to your project

Running `el init` and `el compile` in a LaTeX project creates three things, all gitignored by default:

- **`.el.json`** — stores which `.tex` file is the main document
- **`.aux_dir/`** — all pdflatex/bibtex/biber intermediate files go here instead of cluttering the project root
- **`<name>.pdf`** — a symlink pointing into `.aux_dir/`; named after your main `.tex` file; open this in your PDF viewer
