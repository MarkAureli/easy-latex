package pedantic

import "testing"

func TestHasInlineMath(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{`$x+y$`, true},
		{`text $a$ more`, true},
		{`\(x^2\)`, true},
		{`text \(a+b\) end`, true},
		{`$$x+y$$`, false},
		{`\[x+y\]`, false},
		{`no math here`, false},
		{``, false},
		{`just a dollar sign \$5`, false},
	}
	for _, tt := range tests {
		if got := hasInlineMath(tt.line); got != tt.want {
			t.Errorf("hasInlineMath(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestCheckMathLinebreak(t *testing.T) {
	stx := &SynctexData{
		Inputs: map[int]string{1: "main.tex"},
		Maths: []MathRecord{
			// Pair 1: same v → ok
			{Input: 1, Line: 1, H: 100, V: 200},
			{Input: 1, Line: 1, H: 300, V: 200},
			// Pair 2: different v → line break
			{Input: 1, Line: 3, H: 100, V: 200},
			{Input: 1, Line: 3, H: 300, V: 500},
			// Pair 3: different v but source has no inline math → skip
			{Input: 1, Line: 5, H: 100, V: 200},
			{Input: 1, Line: 5, H: 300, V: 500},
		},
	}
	sources := map[string][]string{
		"main.tex": {
			`text $x+y$ end`,          // line 1
			`no math`,                  // line 2
			`long $a+b+c+d+e+f$ here`, // line 3
			`also no math`,             // line 4
			`just text`,                // line 5 — no inline math
		},
	}

	diags := checkMathLinebreak(stx, sources)
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	if diags[0].Line != 3 {
		t.Errorf("diagnostic line = %d, want 3", diags[0].Line)
	}
}
