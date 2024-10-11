package cio

import "errors"

func (e *editor) multilineMode() (modefunc, EditResult, error) {
	var (
		lines   []string
		offsets []int
	)
	cleanTerminal()
	defer func() {
		move(0, 0)
		cleanTerminal()
	}()
	for {
		buf := newScreenBuf(Width - 1)
		buf.writeAt(0, 0, colorLabel, e.field.Label)
		value := e.value
		if e.field.HideValue {
			value = hideValue(value)
		}
		lines, offsets = wrap(value, Width-5)
		for i, line := range lines {
			buf.fill(4, Width-5, i+1, colorEntry)
			pre, sel, post := splitOnSelect(line, e.sels-offsets[i], e.sele-offsets[i])
			buf.writeAt(4, i+1, colorEntry, pre)
			buf.write(colorSelected, sel)
			buf.write(colorEntry, post)
		}
		paintBuf(buf)
		move(cursorToXY(e.cursor, offsets))
		switch key := readKey(); key {
		case 0:
			return nil, 0, errors.New("error reading stdin")
		case 0x01, keyHome: // Ctrl-A
			e.cursor = offsets[lineContaining(e.cursor, offsets)]
			e.sels, e.sele = e.cursor, e.cursor
		case keyShiftHome:
			e.cursor = offsets[lineContaining(e.cursor, offsets)]
			e.sels = e.cursor
		case 0x02, keyLeft: // Ctrl-B
			if e.cursor > 0 {
				e.cursor--
			}
			e.sels, e.sele = e.cursor, e.cursor
		case keyShiftLeft:
			if e.sels != e.sele && e.sele == e.cursor {
				e.sele, e.cursor = e.sele-1, e.cursor-1
			} else if e.cursor > 0 {
				e.sels, e.cursor = e.sels-1, e.cursor-1
			}
		case keyCtrlLeft:
			e.cursor = prevword(e.value, e.cursor)
			e.sels, e.sele = e.cursor, e.cursor
		case keyCtrlShiftLeft:
			if e.sels != e.sele && e.sele == e.cursor {
				e.cursor = prevword(e.value, e.cursor)
				e.sele = e.cursor
				if e.sele < e.sels {
					e.sels = e.sele
				}
			} else {
				e.cursor = prevword(e.value, e.cursor)
				e.sels = e.cursor
			}
		case 0x03: // Ctrl-C
			return nil, 0, errors.New("interrupted")
		case 0x04, keyDelete: // Ctrl-D
			if len(e.value) > e.cursor {
				e.value = e.value[:e.cursor] + e.value[e.cursor+1:]
				e.changed = true
			}
			e.sels, e.sele = e.cursor, e.cursor
		case 0x05, keyEnd: // Ctrl-E, End
			line := lineContaining(e.cursor, offsets)
			e.cursor = min(offsets[line+1], len(e.value))
			if e.cursor > 0 && e.value[e.cursor-1] == '\n' {
				e.cursor--
			}
			e.sels, e.sele = e.cursor, e.cursor
		case keyShiftEnd:
			line := lineContaining(e.cursor, offsets)
			e.cursor = min(offsets[line+1], len(e.value))
			if e.cursor > 0 && e.value[e.cursor-1] == '\n' {
				e.cursor--
			}
			e.sele = e.cursor
		case 0x06, keyRight: // Ctrl-F
			if e.cursor < len(e.value) {
				e.cursor++
			}
			e.sels, e.sele = e.cursor, e.cursor
		case keyShiftRight:
			if e.sels != e.sele && e.sels == e.cursor {
				e.sels, e.cursor = e.sels+1, e.cursor+1
			} else if e.cursor < len(e.value) {
				e.sele, e.cursor = e.sele+1, e.cursor+1
			}
		case keyCtrlRight:
			e.cursor = nextword(e.value, e.cursor)
			e.sels, e.sele = e.cursor, e.cursor
		case keyCtrlShiftRight:
			if e.sels != e.sele && e.sels == e.cursor {
				e.cursor = nextword(e.value, e.cursor)
				e.sels = e.cursor
				if e.sels < e.sele {
					e.sele = e.sels
				}
			} else {
				e.cursor = nextword(e.value, e.cursor)
				e.sele = e.cursor
			}
		case 0x08, 0x7f: // Backspace
			if e.sels != e.sele {
				e.value = e.value[:e.sels] + e.value[e.sele:]
				e.cursor, e.changed = e.sels, true
			} else if e.cursor > 0 {
				e.value = e.value[:e.cursor-1] + e.value[e.cursor:]
				e.cursor, e.changed = e.cursor-1, true
			}
			e.sels, e.sele = e.cursor, e.cursor
		case 0x09: // Tab
			return nil, ResultNext, nil
		case keyBackTab:
			return nil, ResultPrevious, nil
		case 0x0A, 0x0D: // Enter
			if e.cursor == 0 && e.value == "" {
				continue // ignore newlines in empty field
			}
			if e.cursor == len(e.value) && e.cursor > 0 && e.sele == e.cursor && e.sels == 0 {
				// Enter when the entire content of the field is
				// selected.  They're probably just trying to
				// skip the entire field.
				return nil, ResultNext, nil
			}
			if e.cursor == len(e.value) && e.cursor > 1 && e.value[e.cursor-1] == '\n' && e.value[e.cursor-2] == '\n' {
				// Three Enters in a row: they're probably
				// trying to move on to the next field and don't
				// know how.
				return nil, ResultNext, nil
			}
			e.value = e.value[:e.sels] + "\n" + e.value[e.sele:]
			e.cursor, e.changed = e.sels+1, true
			e.sels, e.sele = e.cursor, e.cursor
		case 0x0B: // Ctrl-K
			line := lineContaining(e.cursor, offsets)
			eol := min(offsets[line+1], len(e.value))
			if eol > 0 && e.value[eol-1] == '\n' {
				eol--
			}
			if e.cursor != eol {
				e.value = e.value[:e.cursor] + e.value[eol:]
				e.changed = true
			}
			e.sels, e.sele = e.cursor, e.cursor
		case 0x0C: // Ctrl-L
			return e.multilineMode, 0, nil
		case 0x0E, keyDown: // Ctrl-N
			line := lineContaining(e.cursor, offsets)
			if line == len(lines)-1 {
				e.cursor = len(e.value)
			} else {
				e.cursor += offsets[line+1] - offsets[line]
				if e.cursor >= offsets[line+2] {
					e.cursor = offsets[line+2] - 1
				}
			}
			e.sels, e.sele = e.cursor, e.cursor
		case keyShiftDown:
			line := lineContaining(e.cursor, offsets)
			if line == len(lines)-1 {
				e.cursor = len(e.value)
			} else {
				e.cursor += offsets[line+1] - offsets[line]
				if e.cursor >= offsets[line+2] {
					e.cursor = offsets[line+2] - 1
				}
			}
			e.sele = e.cursor
		case 0x10, keyUp: // Ctrl-P
			line := lineContaining(e.cursor, offsets)
			if line == 0 {
				e.cursor = 0
			} else {
				e.cursor -= offsets[line] - offsets[line-1]
				if e.cursor >= offsets[line] {
					e.cursor = offsets[line] - 1
				}
			}
			e.sels, e.sele = e.cursor, e.cursor
		case keyShiftUp:
			line := lineContaining(e.cursor, offsets)
			if line == 0 {
				e.cursor = 0
			} else {
				e.cursor -= offsets[line] - offsets[line-1]
				if e.cursor >= offsets[line] {
					e.cursor = offsets[line] - 1
				}
			}
			e.sels = e.cursor
		case 0x15: // Ctrl-U
			if e.value != "" {
				e.value, e.changed = "", true
			}
			e.sels, e.sele, e.cursor = 0, 0, 0
		case 0x1B:
			return nil, ResultDone, nil
		case keyF1:
			e.showHelp()
			return e.multilineMode, 0, nil
		default:
			if key >= 0x20 && key <= 0x7e { // Printable character
				e.value = e.value[:e.sels] + string(key) + e.value[e.sele:]
				e.cursor, e.changed = e.sels+1, true
				e.sels, e.sele = e.cursor, e.cursor
			}
		}
	}
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
