package editfield

import "strings"

func (e *Editor) multilineMode() (modefunc, Result) {
	var (
		lines   []string
		offsets []int
	)
	// Draw the label.
	e.x, e.y = 0, 0
	e.out(resetAndClearEOS, 0)
	defer e.out(showCursor, 0)
	e.out(fieldLabelStyle, 0)
	e.out(e.label, len(e.label))
	e.out(normalStyle, 0)
	// Draw the initial value and entry area.
	lines, offsets = wrap(e.value, e.screenWidth-5)
	for i, line := range lines {
		var presel, sel, postsel string

		e.newline()
		e.out("    ", 4)
		presel, sel, postsel = splitOnSelect(line, e.sels-offsets[i], e.sele-offsets[i])
		e.out(entryStyle, 0)
		if presel != "" {
			e.out(presel, len(presel))
		}
		if sel != "" {
			e.out(selectedStyle, 0)
			e.out(sel, len(sel))
			e.out(entryStyle, 0)
		}
		if postsel != "" {
			e.out(postsel, len(sel))
		}
		if len(line) < e.screenWidth-5 {
			e.out(spaces[:e.screenWidth-len(line)-5], 0)
			e.x = e.screenWidth - 1
		}
		e.out(normalStyle, 0)
	}
	// Run the input loop.
	for {
		var newstart, newend, newcur, newval = e.sels, e.sele, e.cursor, e.value

		e.move(cursorToXY(e.cursor, offsets))
		switch key := readKey(); key {
		case 0:
			e.move(0, 0)
			return nil, ResultError
		case 0x01, keyHome: // Ctrl-A
			newcur = offsets[lineContaining(newcur, offsets)]
			newstart, newend = newcur, newcur
		case keyShiftHome:
			newcur = offsets[lineContaining(newcur, offsets)]
			newstart = newcur
		case 0x02, keyLeft: // Ctrl-B
			if newcur > 0 {
				newcur--
			}
			newstart, newend = newcur, newcur
		case keyShiftLeft:
			if newstart != newend && newend == newcur {
				newend, newcur = newend-1, newcur-1
			} else if newcur > 0 {
				newstart, newcur = newstart-1, newcur-1
			}
		case keyCtrlLeft:
			newcur = prevword(newval, newcur)
			newstart, newend = newcur, newcur
		case keyCtrlShiftLeft:
			if newstart != newend && newend == newcur {
				newcur = prevword(newval, newcur)
				newend = newcur
				if newend < newstart {
					newstart = newend
				}
			} else {
				newcur = prevword(newval, newcur)
				newstart = newcur
			}
		case 0x03: // Ctrl-C
			e.move(0, 0)
			return nil, ResultCtrlC
		case 0x04, keyDelete: // Ctrl-D
			if len(newval) > newcur {
				newval = newval[:newcur] + newval[newcur+1:]
			}
			newstart, newend = newcur, newcur
		case 0x05, keyEnd: // Ctrl-E, End
			line := lineContaining(newcur, offsets)
			newcur = min(offsets[line+1], len(newval))
			if newcur > 0 && newval[newcur-1] == '\n' {
				newcur--
			}
			newstart, newend = newcur, newcur
		case keyShiftEnd:
			line := lineContaining(newcur, offsets)
			newcur = min(offsets[line+1], len(newval))
			if newcur > 0 && newval[newcur-1] == '\n' {
				newcur--
			}
			newend = newcur
		case 0x06, keyRight: // Ctrl-F
			if newcur < len(newval) {
				newcur++
			}
			newstart, newend = newcur, newcur
		case keyShiftRight:
			if newstart != newend && newstart == newcur {
				newstart, newcur = newstart+1, newcur+1
			} else if newcur < len(newval) {
				newend, newcur = newend+1, newcur+1
			}
		case keyCtrlRight:
			newcur = nextword(newval, newcur)
			newstart, newend = newcur, newcur
		case keyCtrlShiftRight:
			if newstart != newend && newstart == newcur {
				newcur = nextword(newval, newcur)
				newstart = newcur
				if newstart < newend {
					newend = newstart
				}
			} else {
				newcur = nextword(newval, newcur)
				newend = newcur
			}
		case 0x08, 0x7f: // Backspace
			if newcur > 0 {
				newval = newval[:newcur-1] + newval[newcur:]
				newcur--
			}
			newstart, newend = newcur, newcur
		case 0x09: // Tab
			e.move(0, 0)
			return nil, ResultTab
		case keyBackTab:
			e.move(0, 0)
			return nil, ResultBackTab
		case 0x0A, 0x0D: // Enter
			if newcur == 0 && newval == "" {
				continue // ignore newlines in empty field
			}
			if newcur == len(newval) && newcur > 1 && newval[newcur-1] == '\n' && newval[newcur-2] == '\n' {
				// Three Enters in a row: they're probably
				// trying to move on to the next field and don't
				// know how.
				e.move(0, 0)
				return nil, ResultEnter
			}
			newval = newval[:newstart] + "\n" + newval[newend:]
			newcur = newstart + 1
			newstart, newend = newcur, newcur
		case 0x0B: // Ctrl-K
			line := lineContaining(newcur, offsets)
			eol := min(offsets[line+1], len(newval))
			if eol > 0 && newval[eol-1] == '\n' {
				eol--
			}
			newval = newval[:newcur] + newval[eol:]
			newstart, newend = newcur, newcur
		case 0x0C: // Ctrl-L
			e.move(0, 0)
			return e.multilineMode, 0
		case 0x0E, keyDown: // Ctrl-N
			line := lineContaining(newcur, offsets)
			if line == len(lines)-1 {
				newcur = len(newval)
			} else {
				newcur += offsets[line+1] - offsets[line]
				if newcur >= offsets[line+2] {
					newcur = offsets[line+2] - 1
				}
			}
			newstart, newend = newcur, newcur
		case keyShiftDown:
			line := lineContaining(newcur, offsets)
			if line == len(lines)-1 {
				newcur = len(newval)
			} else {
				newcur += offsets[line+1] - offsets[line]
				if newcur >= offsets[line+2] {
					newcur = offsets[line+2] - 1
				}
			}
			newend = newcur
		case 0x10, keyUp: // Ctrl-P
			line := lineContaining(newcur, offsets)
			if line == 0 {
				newcur = 0
			} else {
				newcur -= offsets[line] - offsets[line-1]
				if newcur >= offsets[line] {
					newcur = offsets[line] - 1
				}
			}
			newstart, newend = newcur, newcur
		case keyShiftUp:
			line := lineContaining(newcur, offsets)
			if line == 0 {
				newcur = 0
			} else {
				newcur -= offsets[line] - offsets[line-1]
				if newcur >= offsets[line] {
					newcur = offsets[line] - 1
				}
			}
			newstart = newcur
		case 0x15: // Ctrl-U
			newval = ""
			newstart, newend, newcur = 0, 0, 0
		case 0x1B:
			e.move(0, 0)
			return nil, ResultESC
		case keyF1:
			e.move(0, 0)
			e.showHelp()
			return e.multilineMode, 0
		default:
			if key >= 0x20 && key <= 0x7e { // Printable character
				newval = newval[:newstart] + string(key) + newval[newend:]
				newcur = newstart + 1
				newstart, newend = newcur, newcur
			}
		}
		// Update each line that needs updating.
		newlines, newoffsets := wrap(newval, e.screenWidth-5)
		for len(newlines) < len(lines) {
			newlines = append(newlines, "")
			newoffsets = append(newoffsets, newoffsets[len(newoffsets)-1])
		}
		for i, newline := range newlines {
			var line string
			var offset int
			if i < len(lines) {
				line, offset = lines[i], offsets[i]
			} else {
				e.move(e.screenWidth-1, i)
				e.newline()
			}
			var o1, o2, o3 = splitOnSelect(line, e.sels-offset, e.sele-offset)
			var n1, n2, n3 = splitOnSelect(newline, newstart-newoffsets[i], newend-newoffsets[i])
			if o1 == n1 && o2 == n2 && o3 == n3 && i < len(lines) {
				continue // no change
			}
			e.changed = true
			e.out(entryStyle, 0)
			if o1 != n1 {
				e.move(4, i+1)
				e.out(n1, len(n1))
			}
			if o2 != n2 || o1 != n1 {
				e.move(4+len(n1), i+1)
				e.out(selectedStyle, 0)
				e.out(n2, len(n2))
				e.out(entryStyle, 0)
			}
			if o3 != n3 || o2 != n2 || o1 != n1 {
				e.move(4+len(n1)+len(n2), i+1)
				e.out(n3, len(n3))
			}
			e.move(4+len(n1)+len(n2)+len(n3), i+1)
			e.out(spaces[:e.screenWidth-len(newline)-5], 0)
			e.x = e.screenWidth - 1
			e.out(normalStyle, 0)
		}
		// Apply the changes.
		e.value, e.cursor, e.sels, e.sele = newval, newcur, newstart, newend
		lines, offsets = newlines, newoffsets
	}
}

func splitOnSelect(s string, selstart, selend int) (presel, sel, postsel string) {
	if selstart == selend {
		return s, "", ""
	}
	if selstart >= len(s) {
		presel = s
	} else if selstart >= 0 {
		presel = s[:selstart]
	}
	if selstart < len(s) && selend > 0 {
		if selstart > 0 && selend < len(s) {
			sel = s[selstart:selend]
		} else if selstart > 0 {
			sel = s[selstart:]
		} else {
			sel = s[:selend]
		}
	}
	if selend <= 0 {
		postsel = s
	} else if selend < len(s) {
		postsel = s[selend:]
	}
	if s != presel+sel+postsel {
		panic("assert")
	}
	return presel, sel, postsel
}

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
				offsets = append(offsets, width)
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

func cursorToXY(cursor int, offsets []int) (x, y int) {
	line := lineContaining(cursor, offsets)
	return cursor - offsets[line] + 4, line + 1
}

func lineContaining(offset int, offsets []int) int {
	for i, o := range offsets {
		if o > offset {
			return i - 1
		}
	}
	return len(offsets) - 2
}
