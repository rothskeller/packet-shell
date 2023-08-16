package cio

import (
	"bytes"
)

// A screenBuf is a picture of what the bottom of the terminal screen looks
// like.  It contains a list of lines.  Each line has a slice of characters
// and a parallel slice of the colors with which those characters should be
// drawn.
type screenBuf struct {
	lines []screenBufLine
	x, y  int
}
type screenBufLine struct {
	chars  []byte
	colors []uint16
}

// newScreenBuf returns a new screenBuf with a single empty line of the
// specified width.
func newScreenBuf(width int) *screenBuf {
	return &screenBuf{lines: []screenBufLine{{
		chars:  bytes.Repeat([]byte{' '}, width),
		colors: make([]uint16, width),
	}}}
}

// writeAt writes a string at a particular location in the buffer with a
// specified color.
func (b *screenBuf) writeAt(x, y, color int, text string) {
	for y >= len(b.lines) {
		b.lines = append(b.lines, screenBufLine{
			chars:  bytes.Repeat([]byte{' '}, len(b.lines[0].chars)),
			colors: make([]uint16, len(b.lines[0].chars)),
		})
	}
	if x+len(text) > len(b.lines[y].chars) {
		text = text[:len(b.lines[y].chars)-x]
	}
	copy(b.lines[y].chars[x:], []byte(text))
	for i := range text {
		b.lines[y].colors[x+i] = uint16(color)
	}
	b.x, b.y = x+len(text), y
}

// write writes a string at the current location in the buffer with a specified
// color
func (b *screenBuf) write(color int, text string) {
	b.writeAt(b.x, b.y, color, text)
}

// fill fills an area in the buffer with a specified color.
func (b *screenBuf) fill(x, dx, y, color int) {
	b.writeAt(x, y, color, spaces[:dx])
}

// paintBuf updates the contents of the screen to match the provided buffer.
func paintBuf(n *screenBuf) {
	// If the screen buffer hasn't been used yet, initialize it.
	if buf == nil {
		buf = newScreenBuf(len(n.lines[0].chars))
	}
	// Handle each line separately.
	for y, nline := range n.lines {
		// If the screen doesn't have this line, add it to the screen
		// buffer, and emit a newline at the bottom of the screen to
		// scroll it up.
		if y >= len(buf.lines) {
			buf.fill(0, len(n.lines[0].chars), y, 0)
			move(0, y-1)
			print(0, "\n")
		}
		oline := buf.lines[y]
		// Compare the new buffer line and the screen buffer line.
		var diff = make([]bool, len(nline.chars))
		for x := range nline.chars {
			diff[x] = nline.chars[x] != oline.chars[x] || nline.colors[x] != oline.colors[x]
		}
		// Loop through the comparison looking for runs of differences.
		var x = 0
		for x < len(nline.chars) {
			if !diff[x] {
				x++
				continue
			}
			// We have found a difference.
			startX := x
			// Now, look for a run of at least seven characters
			// without a difference.  That's how many characters are
			// used in a cursor movement escape sequence, so if
			// there are fewer differences than that, it's more
			// efficient to rewrite the unchanged characters than to
			// move the cursor.
			endX := x
			for x < len(nline.chars) && x-endX < 7 {
				if diff[x] {
					endX = x
				}
				x++
			}
			// Paint the difference.
			endX++ // change from inclusive end to exclusive end
			paintLine(startX, endX, y, nline)
		}
		// Now that they match, update the screen buffer to
		// the correct contents.
		copy(oline.chars, nline.chars)
		copy(oline.colors, nline.colors)
	}
	// Handle the case where the new buffer has fewer lines than the screen
	// buffer.
	for len(buf.lines) > len(n.lines) {
		blank := screenBufLine{
			chars:  bytes.Repeat([]byte{' '}, len(buf.lines[0].chars)),
			colors: make([]uint16, len(buf.lines[0].chars)),
		}
		paintLine(0, len(blank.chars), len(buf.lines)-1, blank)
		buf.lines = buf.lines[:len(buf.lines)-1]
	}
}

func paintLine(startX, endX, y int, line screenBufLine) {
	x := startX
	for x < endX {
		// Find the end of the span of characters with the same color.
		var color = line.colors[x]
		var spanStart = x
		move(x, y)
		for x < endX && line.colors[x] == color {
			x++
		}
		// Write those characters with that color.
		printb(int(color), line.chars[spanStart:x])
	}
}
