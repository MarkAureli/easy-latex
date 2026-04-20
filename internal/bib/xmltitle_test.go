package bib

import "testing"

func TestCleanCrossrefTitle(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no xml passthrough",
			in:   "A plain title with no markup",
			want: "A plain title with no markup",
		},
		{
			name: "simple mi identifier with surrounding text",
			in:   `Recurrence properties of unbiased coined quantum walks on infinite<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML" display="inline"><mml:mi>d</mml:mi></mml:math>-dimensional lattices`,
			want: `Recurrence properties of unbiased coined quantum walks on infinite $d$-dimensional lattices`,
		},
		{
			name: "msub subscript",
			in:   `The <mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:msub><mml:mi>R</mml:mi><mml:mn>2</mml:mn></mml:msub></mml:math> group`,
			want: `The $R_{2}$ group`,
		},
		{
			name: "msup superscript",
			in:   `Value of <mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:msup><mml:mi>e</mml:mi><mml:mn>2</mml:mn></mml:msup></mml:math> is large`,
			want: `Value of $e^{2}$ is large`,
		},
		{
			name: "msubsup",
			in:   `<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:msubsup><mml:mi>x</mml:mi><mml:mn>0</mml:mn><mml:mn>2</mml:mn></mml:msubsup></mml:math>`,
			want: `$x_{0}^{2}$`,
		},
		{
			name: "mfrac fraction",
			in:   `A <mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mfrac><mml:mn>1</mml:mn><mml:mn>2</mml:mn></mml:mfrac></mml:math> spin`,
			want: `A $\frac{1}{2}$ spin`,
		},
		{
			name: "msqrt",
			in:   `<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:msqrt><mml:mn>2</mml:mn></mml:msqrt></mml:math>`,
			want: `$\sqrt{2}$`,
		},
		{
			name: "nested mrow wrappers",
			in:   `<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mrow><mml:msub><mml:mrow><mml:mi>R</mml:mi></mml:mrow><mml:mrow><mml:mn>2</mml:mn></mml:mrow></mml:msub></mml:mrow></mml:math>`,
			want: `$R_{2}$`,
		},
		{
			name: "mathvariant normal",
			in:   `<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mi mathvariant="normal">Fe</mml:mi></mml:math>`,
			want: `$\mathrm{Fe}$`,
		},
		{
			name: "multiple math blocks",
			in:   `The <mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mi>x</mml:mi></mml:math> and <mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mi>y</mml:mi></mml:math> values`,
			want: `The $x$ and $y$ values`,
		},
		{
			name: "unicode greek in mi",
			in:   `<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mi>` + "\u03B1" + `</mml:mi></mml:math>`,
			want: `$\alpha$`,
		},
		{
			name: "unicode operator in mo",
			in:   `<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mrow><mml:mi>a</mml:mi><mml:mo>` + "\u00d7" + `</mml:mo><mml:mi>b</mml:mi></mml:mrow></mml:math>`,
			want: `$a\times b$`,
		},
		{
			name: "mtext inside math",
			in:   `<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mtext>const</mml:mtext></mml:math>`,
			want: `$\text{const}$`,
		},
		{
			name: "mover hat accent",
			in:   `<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mover><mml:mi>x</mml:mi><mml:mo>` + "\u0302" + `</mml:mo></mml:mover></mml:math>`,
			want: `$\hat{x}$`,
		},
		// Face markup
		{
			name: "italic",
			in:   "Study of <i>Drosophila</i> genes",
			want: `Study of \textit{Drosophila} genes`,
		},
		{
			name: "bold",
			in:   "<b>Important</b> result",
			want: `\textbf{Important} result`,
		},
		{
			name: "subscript face markup",
			in:   "CO<sub>2</sub> emissions",
			want: `CO\textsubscript{2} emissions`,
		},
		{
			name: "superscript face markup",
			in:   "x<sup>2</sup> + 1",
			want: `x\textsuperscript{2} + 1`,
		},
		{
			name: "small caps",
			in:   "indigenous <scp>Bahamian</scp> islanders",
			want: `indigenous \textsc{Bahamian} islanders`,
		},
		{
			name: "mixed mathml and face markup",
			in:   `<i>Drosophila</i> and <mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mi>n</mml:mi></mml:math> genes`,
			want: `\textit{Drosophila} and $n$ genes`,
		},
		{
			name: "unknown tags stripped",
			in:   "some <unknown>text</unknown> here",
			want: "some text here",
		},
		{
			name: "space before math adjacent to letter",
			in:   `word<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mi>x</mml:mi></mml:math>`,
			want: `word $x$`,
		},
		{
			name: "space after math adjacent to letter",
			in:   `<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mi>x</mml:mi></mml:math>word`,
			want: `$x$ word`,
		},
		{
			name: "no space before hyphen",
			in:   `<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mi>d</mml:mi></mml:math>-dim`,
			want: `$d$-dim`,
		},
		{
			name: "no space after open paren",
			in:   `(<mml:math xmlns:mml="http://www.w3.org/1998/Math/MathML"><mml:mi>d</mml:mi></mml:math>)`,
			want: `($d$)`,
		},
		{
			name: "without namespace prefix",
			in:   `<math><mi>x</mi></math>`,
			want: `$x$`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := cleanCrossrefTitle(c.in)
			if got != c.want {
				t.Errorf("cleanCrossrefTitle(%q)\n  got  %q\n  want %q", c.in, got, c.want)
			}
		})
	}
}

func TestGenerateKeyWithMathMLTitle(t *testing.T) {
	e := Entry{
		Type: "article",
		Key:  "original",
		Fields: []Field{
			{Name: "author", Value: `{Štefaňák, M. and Kiss, T. and Jex, I.}`},
			{Name: "year", Value: "{2008}"},
			{Name: "title", Value: `{Recurrence properties of unbiased coined quantum walks on infinite $d$-dimensional lattices}`},
		},
	}
	key := GenerateKey(e)
	want := "Stefanak2008RecurrencePropertiesOfUnbiasedCoinedQuantumWalksOnInfiniteDimensionalLattices"
	if key != want {
		t.Errorf("GenerateKey with cleaned title:\n  got  %q\n  want %q", key, want)
	}
}
