package lsp

// byteToUTF16 converts a byte offset within line to a UTF-16 code-unit
// offset, as required by the LSP `Position.character` field. byteOff is
// clamped to [0, len(line)]. Pure-ASCII lines are unchanged.
func byteToUTF16(line string, byteOff int) int {
	if byteOff <= 0 {
		return 0
	}
	if byteOff > len(line) {
		byteOff = len(line)
	}
	units := 0
	for i, r := range line {
		if i >= byteOff {
			return units
		}
		if r <= 0xFFFF {
			units++
		} else {
			units += 2 // surrogate pair
		}
	}
	return units
}
