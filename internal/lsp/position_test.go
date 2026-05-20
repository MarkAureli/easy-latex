package lsp

import "testing"

func TestByteToUTF16(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		byteOff int
		want    int
	}{
		{"ascii-zero", "hello", 0, 0},
		{"ascii-mid", "hello", 3, 3},
		{"ascii-end", "hello", 5, 5},
		{"ascii-past-end", "hello", 99, 5},
		{"negative", "hello", -1, 0},
		// é = 2 bytes UTF-8, 1 unit UTF-16
		{"latin-after-accent", "café x", 5, 4}, // through 'é '
		{"latin-end", "café", 5, 4},
		// ü = 2 bytes UTF-8, 1 unit UTF-16
		{"german", "Bußmann", 7, 6}, // through full word: B,u,ß,m,a,n = 6 units (ß is 2 bytes), then n
		// emoji 🔥 = 4 bytes UTF-8, 2 units UTF-16 (surrogate pair)
		{"emoji-before", "x🔥y", 1, 1},
		{"emoji-after", "x🔥y", 5, 3}, // through the emoji
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := byteToUTF16(c.line, c.byteOff)
			if got != c.want {
				t.Errorf("byteToUTF16(%q, %d) = %d, want %d", c.line, c.byteOff, got, c.want)
			}
		})
	}
}
