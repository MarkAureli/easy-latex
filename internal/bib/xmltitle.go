package bib

import (
	"encoding/xml"
	"regexp"
	"strings"
)

// cleanCrossrefTitle converts MathML and Crossref face markup in a title
// string to LaTeX equivalents, then strips any remaining XML tags.
func cleanCrossrefTitle(title string) string {
	if !strings.Contains(title, "<") {
		return title
	}
	s := convertMathML(title)
	s = convertFaceMarkup(s)
	s = reAnyTag.ReplaceAllString(s, "")
	return s
}

// ── MathML → LaTeX ──────────────────────────────────────────────────────────

var reMathMLBlock = regexp.MustCompile(`(?s)<(?:mml:)?math\b[^>]*>.*?</(?:mml:)?math>`)

// convertMathML replaces every <(mml:)?math ...>...</math> block with its
// LaTeX $...$ equivalent, inserting a space before/after when adjacent to a
// letter or digit so that the math block reads naturally in running text.
func convertMathML(s string) string {
	locs := reMathMLBlock.FindAllStringIndex(s, -1)
	if locs == nil {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	prev := 0
	for _, loc := range locs {
		sb.WriteString(s[prev:loc[0]])
		latex := mathMLToLaTeX(s[loc[0]:loc[1]])
		if latex == "" {
			prev = loc[1]
			continue
		}
		// Insert space before $...$ if preceded by a letter/digit.
		if loc[0] > 0 && isAlphaNum(s[loc[0]-1]) {
			sb.WriteByte(' ')
		}
		sb.WriteByte('$')
		sb.WriteString(latex)
		sb.WriteByte('$')
		// Insert space after $...$ if followed by a letter/digit.
		if loc[1] < len(s) && isAlphaNum(s[loc[1]]) {
			sb.WriteByte(' ')
		}
		prev = loc[1]
	}
	sb.WriteString(s[prev:])
	return sb.String()
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// mathMLToLaTeX parses a complete <math>...</math> XML fragment and returns
// the LaTeX math-mode body (without delimiters).
func mathMLToLaTeX(fragment string) string {
	d := xml.NewDecoder(strings.NewReader(fragment))
	// Consume the opening <math> element.
	for {
		tok, err := d.Token()
		if err != nil {
			return ""
		}
		if se, ok := tok.(xml.StartElement); ok {
			return processChildren(d, se.End())
		}
	}
}

// processElement converts a single MathML element (already consumed as
// StartElement) into LaTeX.
func processElement(d *xml.Decoder, start xml.StartElement) string {
	name := start.Name.Local

	switch name {
	case "mrow", "mstyle", "mpadded", "mphantom":
		return processChildren(d, start.End())

	case "mi":
		text := collectText(d, start.End())
		// mathvariant="normal" → upright
		for _, a := range start.Attr {
			if a.Name.Local == "mathvariant" && a.Value == "normal" {
				return `\mathrm{` + mapMathSymbols(text) + `}`
			}
		}
		return mapMathSymbols(text)

	case "mn":
		return collectText(d, start.End())

	case "mo":
		text := collectText(d, start.End())
		return mapOperator(text)

	case "mtext":
		text := collectText(d, start.End())
		return `\text{` + text + `}`

	case "msup":
		children := collectNChildren(d, start.End(), 2)
		return children[0] + "^{" + children[1] + "}"

	case "msub":
		children := collectNChildren(d, start.End(), 2)
		return children[0] + "_{" + children[1] + "}"

	case "msubsup":
		children := collectNChildren(d, start.End(), 3)
		return children[0] + "_{" + children[1] + "}^{" + children[2] + "}"

	case "mfrac":
		children := collectNChildren(d, start.End(), 2)
		return `\frac{` + children[0] + "}{" + children[1] + "}"

	case "msqrt":
		inner := processChildren(d, start.End())
		return `\sqrt{` + inner + "}"

	case "mover":
		children := collectNChildren(d, start.End(), 2)
		if cmd := accentCommand(children[1]); cmd != "" {
			return cmd + "{" + children[0] + "}"
		}
		return children[0]

	case "munder":
		children := collectNChildren(d, start.End(), 2)
		if cmd := underCommand(children[1]); cmd != "" {
			return cmd + "{" + children[0] + "}"
		}
		return children[0]

	case "mspace":
		// Skip to end, emit nothing.
		skipToEnd(d)
		return ""

	default:
		// Unknown element: recurse children as safe fallback.
		return processChildren(d, start.End())
	}
}

// processChildren reads tokens until end and processes child elements,
// concatenating their LaTeX output.
func processChildren(d *xml.Decoder, end xml.EndElement) string {
	var sb strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			return sb.String()
		}
		switch t := tok.(type) {
		case xml.StartElement:
			sb.WriteString(processElement(d, t))
		case xml.CharData:
			sb.WriteString(strings.TrimSpace(string(t)))
		case xml.EndElement:
			if t == end {
				return sb.String()
			}
		}
	}
}

// collectText reads all character data until the matching end element.
func collectText(d *xml.Decoder, end xml.EndElement) string {
	var sb strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			return sb.String()
		}
		switch t := tok.(type) {
		case xml.CharData:
			sb.Write(t)
		case xml.StartElement:
			// Unexpected child element inside a leaf — recurse.
			sb.WriteString(processElement(d, t))
		case xml.EndElement:
			if t == end {
				return sb.String()
			}
		}
	}
}

// collectNChildren reads up to n child elements from inside a parent,
// returning their LaTeX representations. If fewer than n children exist,
// empty strings fill the remaining slots. Text between children is ignored.
func collectNChildren(d *xml.Decoder, end xml.EndElement, n int) []string {
	children := make([]string, n)
	idx := 0
	for {
		tok, err := d.Token()
		if err != nil {
			return children
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if idx < n {
				children[idx] = processElement(d, t)
				idx++
			} else {
				skipToEnd(d)
			}
		case xml.EndElement:
			if t == end {
				return children
			}
		}
	}
}

// skipToEnd discards all tokens until the matching end element at depth 0.
func skipToEnd(d *xml.Decoder) {
	depth := 1
	for depth > 0 {
		tok, err := d.Token()
		if err != nil {
			return
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
}

// ── Operator / symbol maps ──────────────────────────────────────────────────

// mapOperator converts MathML <mo> text content to LaTeX. Unicode math
// operators are mapped to commands; ASCII operators pass through.
func mapOperator(s string) string {
	if repl, ok := operatorMap[s]; ok {
		return repl
	}
	return s
}

var operatorMap = map[string]string{
	"\u2212": "-",
	"\u00d7": `\times `,
	"\u00b7": `\cdot `,
	"\u00b1": `\pm `,
	"\u2213": `\mp `,
	"\u2264": `\le `,
	"\u2265": `\ge `,
	"\u2260": `\ne `,
	"\u226a": `\ll `,
	"\u226b": `\gg `,
	"\u2248": `\approx `,
	"\u221d": `\propto `,
	"\u221e": `\infty `,
	"\u2208": `\in `,
	"\u2209": `\notin `,
	"\u2282": `\subset `,
	"\u2286": `\subseteq `,
	"\u222a": `\cup `,
	"\u2229": `\cap `,
	"\u2192": `\to `,
	"\u2190": `\leftarrow `,
	"\u21d2": `\Rightarrow `,
	"\u21d4": `\Leftrightarrow `,
	"\u2202": `\partial `,
	"\u2207": `\nabla `,
	"\u222b": `\int `,
	"\u2211": `\sum `,
	"\u220f": `\prod `,
	"\u2297": `\otimes `,
	"\u2295": `\oplus `,
}

// mapMathSymbols converts Unicode math symbols in <mi> text to LaTeX commands.
// Multi-character strings and plain ASCII are returned unchanged.
func mapMathSymbols(s string) string {
	runes := []rune(s)
	if len(runes) != 1 {
		return s
	}
	if repl, ok := mathSymbolMap[runes[0]]; ok {
		return repl
	}
	return s
}

var mathSymbolMap = map[rune]string{
	'\u2113': `\ell`,
	'\u210F': `\hbar`,
	'\u2135': `\aleph`,
	'\u2202': `\partial`,
	'\u221E': `\infty`,
	'\u2207': `\nabla`,
	// Greek lowercase
	'\u03B1': `\alpha`,
	'\u03B2': `\beta`,
	'\u03B3': `\gamma`,
	'\u03B4': `\delta`,
	'\u03B5': `\varepsilon`,
	'\u03B6': `\zeta`,
	'\u03B7': `\eta`,
	'\u03B8': `\theta`,
	'\u03B9': `\iota`,
	'\u03BA': `\kappa`,
	'\u03BB': `\lambda`,
	'\u03BC': `\mu`,
	'\u03BD': `\nu`,
	'\u03BE': `\xi`,
	'\u03C0': `\pi`,
	'\u03C1': `\rho`,
	'\u03C3': `\sigma`,
	'\u03C4': `\tau`,
	'\u03C5': `\upsilon`,
	'\u03C6': `\varphi`,
	'\u03C7': `\chi`,
	'\u03C8': `\psi`,
	'\u03C9': `\omega`,
	// Greek uppercase
	'\u0393': `\Gamma`,
	'\u0394': `\Delta`,
	'\u0398': `\Theta`,
	'\u039B': `\Lambda`,
	'\u039E': `\Xi`,
	'\u03A0': `\Pi`,
	'\u03A3': `\Sigma`,
	'\u03A6': `\Phi`,
	'\u03A8': `\Psi`,
	'\u03A9': `\Omega`,
}

// ── Accent maps ─────────────────────────────────────────────────────────────

// accentCommand maps a MathML <mover> accent character to a LaTeX command.
func accentCommand(accent string) string {
	switch accent {
	case "\u0302", "\u005E":
		return `\hat`
	case "\u0303", "\u007E":
		return `\tilde`
	case "\u0304", "\u00AF", "\u0305":
		return `\bar`
	case "\u0307", "\u002E":
		return `\dot`
	case "\u0308":
		return `\ddot`
	case "\u20D7", "\u2192", "\u279C":
		return `\vec`
	}
	return ""
}

// underCommand maps a MathML <munder> accent character to a LaTeX command.
func underCommand(accent string) string {
	switch accent {
	case "\u0332", "_":
		return `\underline`
	case "\u23DF":
		return `\underbrace`
	}
	return ""
}

// ── Face markup ─────────────────────────────────────────────────────────────

var faceMarkupRules = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?s)<i>(.*?)</i>`), `\textit{$1}`},
	{regexp.MustCompile(`(?s)<b>(.*?)</b>`), `\textbf{$1}`},
	{regexp.MustCompile(`(?s)<sub>(.*?)</sub>`), `\textsubscript{$1}`},
	{regexp.MustCompile(`(?s)<sup>(.*?)</sup>`), `\textsuperscript{$1}`},
	{regexp.MustCompile(`(?s)<scp>(.*?)</scp>`), `\textsc{$1}`},
	{regexp.MustCompile(`(?s)<tt>(.*?)</tt>`), `\texttt{$1}`},
	{regexp.MustCompile(`(?s)<u>(.*?)</u>`), `\underline{$1}`},
	{regexp.MustCompile(`(?s)<ovl>(.*?)</ovl>`), `\overline{$1}`},
}

func convertFaceMarkup(s string) string {
	for _, r := range faceMarkupRules {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}

// ── Safety net ──────────────────────────────────────────────────────────────

var reAnyTag = regexp.MustCompile(`<[^>]+>`)
