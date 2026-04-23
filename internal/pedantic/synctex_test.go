package pedantic

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func writeSynctexGz(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "test.synctex.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	gz.Write([]byte(content))
	gz.Close()
	f.Close()
	return path
}

func TestParseSynctex(t *testing.T) {
	content := `SyncTeX Version:1
Input:1:./main.tex
Input:2:./other.tex
Output:pdf
Magnification:1000
Unit:1
X Offset:0
Y Offset:0
Content:
{1
$1,3:100,200
$1,3:300,200
$2,5:400,500
$2,5:600,700
}1
`
	dir := t.TempDir()
	path := writeSynctexGz(t, dir, content)

	data, err := ParseSynctex(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := data.InputFile(1); got != "./main.tex" {
		t.Errorf("InputFile(1) = %q, want ./main.tex", got)
	}
	if got := data.InputFile(2); got != "./other.tex" {
		t.Errorf("InputFile(2) = %q, want ./other.tex", got)
	}
	if len(data.Maths) != 4 {
		t.Fatalf("got %d math records, want 4", len(data.Maths))
	}

	pairs := data.MathPairs()
	if len(pairs) != 2 {
		t.Fatalf("got %d pairs, want 2", len(pairs))
	}

	// Pair 1: same v → no line break
	if pairs[0].Open.V != pairs[0].Close.V {
		t.Errorf("pair 0: v mismatch %d vs %d, expected same", pairs[0].Open.V, pairs[0].Close.V)
	}
	// Pair 2: different v → line break
	if pairs[1].Open.V == pairs[1].Close.V {
		t.Error("pair 1: expected different v values")
	}
}

func TestParseSynctexOddRecords(t *testing.T) {
	content := `SyncTeX Version:1
Input:1:./main.tex
Content:
{1
$1,3:100,200
$1,3:300,200
$1,5:400,500
}1
`
	dir := t.TempDir()
	path := writeSynctexGz(t, dir, content)

	data, err := ParseSynctex(path)
	if err != nil {
		t.Fatal(err)
	}

	pairs := data.MathPairs()
	if len(pairs) != 1 {
		t.Fatalf("got %d pairs, want 1 (odd record dropped)", len(pairs))
	}
}
