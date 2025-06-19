package cio

import "strings"

// IndentMarker marks the indentation level for wrapped lines in a block of
// running text.  If a line contains this character (before the first wrap)
// point, all wrapped continuations of that line will be indented to the
// location of this character.  This character is not included in the output.
const IndentMarker = '⇥'

// WrapText wraps a string to fit within the screen width.  It gives special
// control over the indentation of wrapped lines.  By default, wrapped lines are
// indented the same amount as the initial unwrapped line.  However, if the
// pre-wrapped line contains a "⇥" character that appears before the first wrap
// point, all wrapped lines are indented to the location of that character
// (which is not included in the output).
func WrapText(s string) (wrapped string) {
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		var indent int

		line = strings.TrimRight(line, " ")
		if indent = strings.IndexRune(line, IndentMarker); indent >= 0 {
			// Remove the indent marker from the line.
			line = line[:indent] + line[indent+3:] // 3 bytes in UTF-8
			if indent >= Width {
				indent = -1
			}
		}
		if indent < 0 {
			indent = strings.IndexFunc(line, func(r rune) bool { return r != ' ' })
			// The only way that comes out -1 is for an empty line,
			// which will never wrap, so its indent doesn't matter.
		}
		for len(line) > Width {
			if idx := strings.LastIndexByte(line[:Width], ' '); idx > 0 {
				wrapped += line[:idx] + "\n"
				line = spaces[:indent] + strings.TrimLeft(line[idx+1:], " ")
			} else {
				wrapped += line[:Width] + "\n"
				line = spaces[:indent] + line[Width:]
			}
		}
		wrapped += line + "\n"
	}
	return wrapped
}

// W67:  [0 2 70] goes to [0 2 67 71]

// wrap wraps a string to fit within the specified width.  It wraps at
// whitespace where possible, and mid-word if necessary.  The returned offsets
// array gives the offset into the original string of the start of each line.
func wrap(s string, width int) (lines []string, offsets []int) {
	offsets = append(offsets, 0)
	offset := 0

	for _, line := range strings.Split(s, "\n") {
		for len(line) > width {
			if idx := strings.LastIndexByte(line[:width], ' '); idx > 0 {
				lines = append(lines, line[:idx+1])
				line = line[idx+1:]
				offset += idx + 1
				offsets = append(offsets, offset)
			} else {
				lines = append(lines, line[:width])
				line = line[width:]
				offset += width
				offsets = append(offsets, offset)
			}
		}
		lines = append(lines, line)
		offset += len(line) + 1 // account for newline
		offsets = append(offsets, offset)
	}
	// Note that this leaves the final offset at len(s)+1, since we
	// accounted for a newline on the final line that isn't actually in the
	// string.  However, this is good, because it means cursorToXY(len(s))
	// will return the end of the last line rather than the start of the
	// nonexistent one.
	return lines, offsets
}
