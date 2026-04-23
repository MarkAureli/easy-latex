package pedantic

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SynctexData holds parsed synctex information.
type SynctexData struct {
	Inputs map[int]string // tag → file path
	Maths  []MathRecord   // all $ records in order
}

// MathRecord is a single synctex $ record (math shift).
type MathRecord struct {
	Input int // input file tag
	Line  int // source line number
	H     int // horizontal position
	V     int // vertical position
}

// MathPair is a sequential pair of math-on / math-off records.
type MathPair struct {
	Open  MathRecord
	Close MathRecord
}

// MathPairs returns sequential pairs of math records.
func (s *SynctexData) MathPairs() []MathPair {
	pairs := make([]MathPair, 0, len(s.Maths)/2)
	for i := 0; i+1 < len(s.Maths); i += 2 {
		pairs = append(pairs, MathPair{Open: s.Maths[i], Close: s.Maths[i+1]})
	}
	return pairs
}

// InputFile returns the file path for the given input tag.
func (s *SynctexData) InputFile(tag int) string {
	return s.Inputs[tag]
}

// ParseSynctex reads and parses a .synctex.gz file.
// Only Input lines and $ (math shift) records are extracted.
func ParseSynctex(path string) (*SynctexData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("decompress %s: %w", path, err)
	}
	defer gz.Close()

	data := &SynctexData{Inputs: make(map[int]string)}
	sc := bufio.NewScanner(gz)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "Input:") {
			parseInputLine(data, line)
		} else if strings.HasPrefix(line, "$") {
			parseMathLine(data, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return data, nil
}

// parseInputLine parses "Input:<tag>:<path>".
func parseInputLine(data *SynctexData, line string) {
	// "Input:1:/path/to/file.tex"
	rest := line[len("Input:"):]
	idx := strings.IndexByte(rest, ':')
	if idx < 0 {
		return
	}
	tag, err := strconv.Atoi(rest[:idx])
	if err != nil {
		return
	}
	data.Inputs[tag] = rest[idx+1:]
}

// parseMathLine parses "$<input>,<line>:<h>,<v>".
func parseMathLine(data *SynctexData, line string) {
	// "$1,169:11322031,10413866"
	rest := line[1:] // skip '$'
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return
	}
	left := rest[:colon]  // "1,169"
	right := rest[colon+1:] // "11322031,10413866"

	comma := strings.IndexByte(left, ',')
	if comma < 0 {
		return
	}
	input, err := strconv.Atoi(left[:comma])
	if err != nil {
		return
	}
	srcLine, err := strconv.Atoi(left[comma+1:])
	if err != nil {
		return
	}

	comma2 := strings.IndexByte(right, ',')
	if comma2 < 0 {
		return
	}
	h, err := strconv.Atoi(right[:comma2])
	if err != nil {
		return
	}
	v, err := strconv.Atoi(right[comma2+1:])
	if err != nil {
		return
	}

	data.Maths = append(data.Maths, MathRecord{
		Input: input,
		Line:  srcLine,
		H:     h,
		V:     v,
	})
}
