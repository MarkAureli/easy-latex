package pedantic

import "testing"

func TestRegionMask_InlineMath(t *testing.T) {
	lines := []string{"Foo $a + b$ bar"}
	m := regionMask(lines)
	// Boundary `$` bytes are regText; only the inner content is regMath.
	for i, want := range []regionKind{
		regText, regText, regText, regText, // "Foo "
		regText,                                     // "$" opener
		regMath, regMath, regMath, regMath, regMath, // "a + b"
		regText,                            // "$" closer
		regText, regText, regText, regText, // " bar"
	} {
		if got := m[0][i]; got != want {
			t.Errorf("byte %d (%q): got %d, want %d", i, lines[0][i], got, want)
		}
	}
}

func TestRegionMask_DisplayMathSpansLines(t *testing.T) {
	lines := []string{
		`text \[`,
		`  a + b`,
		`\] more`,
	}
	m := regionMask(lines)
	if m[1][0] != regMath || m[1][6] != regMath {
		t.Errorf("line 2 should be all math, got %v", m[1])
	}
	// Boundary `\]` bytes are regText; " more" is text too.
	if m[2][0] != regText || m[2][1] != regText {
		t.Error("`\\]` closer bytes should be regText")
	}
	if m[2][2] != regText {
		t.Error("byte after `\\]` should be text")
	}
}

func TestRegionMask_VerbatimEnv(t *testing.T) {
	lines := []string{
		`before \begin{verbatim}`,
		`raw \section content`,
		`\end{verbatim} after`,
	}
	m := regionMask(lines)
	if m[1][0] != regVerbatim {
		t.Error("inside verbatim, line 2 should be verbatim")
	}
	// "after" past \end{verbatim} should be text
	closer := len(`\end{verbatim}`)
	if m[2][closer] != regText {
		t.Errorf("byte %d (%q) after \\end{verbatim} should be text, got %d",
			closer, lines[2][closer], m[2][closer])
	}
}

func TestRegionMask_EscapedDollar(t *testing.T) {
	lines := []string{`\$50 not math`}
	m := regionMask(lines)
	for i, want := range []regionKind{
		regText, regText, // "\$"
		regText, regText, // "50"
		regText, regText, regText, regText, regText, regText, regText, regText, regText, // " not math"
	} {
		if got := m[0][i]; got != want {
			t.Errorf("byte %d (%q): got %d, want %d", i, lines[0][i], got, want)
		}
	}
}

func TestRegionMask_ParenMath(t *testing.T) {
	lines := []string{`a \(x+y\) b`}
	m := regionMask(lines)
	// "a " text; "\(" text-marked opener; "x+y" math; "\)" math-marked closer; " b" text.
	if m[0][2] != regText || m[0][3] != regText {
		t.Error("`\\(` opener bytes should be text")
	}
	if m[0][4] != regMath {
		t.Error("byte after \\( should be math")
	}
	if m[0][7] != regText || m[0][8] != regText {
		t.Error("`\\)` closer bytes should be regText")
	}
	if m[0][9] != regText {
		t.Error("byte after \\) should be text")
	}
}
