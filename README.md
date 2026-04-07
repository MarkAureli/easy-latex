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

### pdflatex not in PATH?

On macOS, TeX Live installs `pdflatex` to `/Library/TeX/texbin/`. `el` checks this location automatically as a fallback, so it works even if you haven't added it to your `$PATH`.

## Project files

| File | Description |
|------|-------------|
| `.el.json` | Project config (main tex file, aux dir). Commit this. |
| `.aux_dir/` | pdflatex auxiliary files. Generated, do not commit. |
| `thesis.pdf` | Symlink to `.aux_dir/thesis.pdf`. Generated, do not commit. |
