package editfield

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const invalid = 99999

func (e *Editor) onelineMode() (modefunc, Result) {
	var (
		fieldWidth int
		entryx     int
		verbatim   bool
	)
	// Draw the label.
	e.x, e.y = 0, 0
	e.out(resetAndClearEOS, 0)
	defer e.out(showCursor, 0)
	e.out(fieldLabelStyle, 0)
	e.out(e.label, len(e.label))
	e.out(normalStyle, 0)
	entryx = e.labelWidth + 2
	if len(e.label) > e.labelWidth {
		entryx = len(e.label) + 2
	}
	e.move(entryx, 0)
	// Draw the value, entry area, and hint.
	e.out(entryStyle, 0)
	e.out(e.value, len(e.value))
	fieldWidth = e.fieldWidth
	if fieldWidth == 0 || entryx+fieldWidth >= e.screenWidth {
		fieldWidth = e.screenWidth - entryx - 1
	}
	if len(e.value) < fieldWidth {
		e.out(spaces[:fieldWidth-len(e.value)], 0)
		e.x = entryx + fieldWidth
		if e.hint != "" && e.x+len(e.hint)+2 < e.screenWidth {
			e.out(hintStyle, 0)
			e.out(spaces[:2], 2)
			e.out(e.hint, len(e.hint))
		}
	}
	e.out(normalStyle, 0)
	// Run the input loop.
	for {
		var newstart, newend, newcur, newval = e.sels, e.sele, e.cursor, e.value
		var diff, oldhlstart, oldhlend, newhlstart, newhlend int

		e.move(entryx+e.cursor, 0)
		switch key := readKey(); key {
		case 0:
			return nil, ResultError
		case 0x01, keyHome: // Ctrl-A
			newstart, newend, newcur = 0, 0, 0
		case keyShiftHome:
			newstart, newcur = 0, 0
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
			return nil, ResultCtrlC
		case 0x04, keyDelete: // Ctrl-D
			if len(newval) > newcur {
				newval = newval[:newcur] + newval[newcur+1:]
			}
			newstart, newend = newcur, newcur
		case 0x05, keyEnd: // Ctrl-E, End
			newcur = len(newval)
			newstart, newend = newcur, newcur
		case keyShiftEnd:
			newcur = len(newval)
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
			return nil, ResultTab
		case keyBackTab:
			return nil, ResultBackTab
		case 0x0A, 0x0D: // Enter
			if verbatim || (newcur != 0 && e.multiline) {
				// Add a literal newline and switch to multiline mode.
				e.value = e.value[:e.sels] + "\n" + e.value[e.sele:]
				e.sels++
				e.cursor, e.sele = e.sels, e.sels
				return e.multilineMode, 0
			}
			return nil, ResultEnter
		case 0x0B: // Ctrl-K
			newval = newval[:newcur]
			newstart, newend = newcur, newcur
		case 0x0C: // Ctrl-L
			return e.onelineMode, 0
		case 0x15: // Ctrl-U
			newval = ""
			newstart, newend, newcur = 0, 0, 0
		case 0x16: // Ctrl-V
			verbatim = true
			continue
		case 0x1B:
			return nil, ResultESC
		case keyF1:
			e.showHelp()
			return e.onelineMode, 0
		default:
			if key >= 0x20 && key <= 0x7e { // Printable character
				newval = newval[:newstart] + string(key) + newval[newend:]
				newcur = newstart + 1
				newstart, newend = newcur, newcur
				if newstart == len(newval) {
					if auto := autocomplete(newval, e.choices); auto != "" {
						newval, newend = auto, len(auto)
					}
				}
				if entryx+len(newval) >= e.screenWidth {
					// No room for value, switch to multiline mode.
					e.value, e.cursor, e.sels, e.sele = newval, newcur, newstart, newend
					return e.multilineMode, 0
				}
			}
		}
		verbatim = false
		// Find the location of the first difference.
		if diff = diffindex(e.value, newval); diff != invalid {
			e.changed = true
		}
		oldhlstart, newhlstart = invalid, invalid
		if e.sels != e.sele {
			oldhlstart, oldhlend = e.sels, e.sele
		}
		if newstart != newend {
			newhlstart, newhlend = newstart, newend
		}
		if oldhlstart != newhlstart {
			diff = min(diff, oldhlstart)
			diff = min(diff, newhlstart)
		}
		if oldhlend != newhlend {
			diff = min(diff, oldhlend)
			diff = min(diff, newhlend)
		}
		// Apply the changes.
		e.value, e.cursor, e.sels, e.sele = newval, newcur, newstart, newend
		if diff == invalid {
			continue // no visual change, don't repaint
		}
		// Repaint the changed portion.
		e.move(entryx+diff, 0)
		e.out(entryStyle, 0)
		if diff < e.sels {
			e.out(e.value[diff:e.sels], e.sels-diff)
		}
		if e.sels != e.sele && diff < e.sele {
			e.out(selectedStyle, 0)
			from := max(diff, e.sels)
			e.out(e.value[from:e.sele], e.sele-from)
			e.out(entryStyle, 0)
		}
		e.out(e.value[e.sele:], len(e.value)-e.sele)
		if len(e.value) < fieldWidth {
			e.out(spaces[:fieldWidth-len(e.value)], 0)
			e.x = entryx + fieldWidth
			if e.hint != "" && e.x+len(e.hint)+2 < e.screenWidth {
				e.out(hintStyle, 0)
				e.out(spaces[:2], 2)
				e.out(e.hint, len(e.hint))
			}
			e.out(normalStyle, 0)
		} else {
			e.out(normalStyle, 0)
			e.out(clearEOL, 0) // remove hint if any
		}
	}
}

func (e *Editor) out(s string, width int) {
	io.WriteString(os.Stdout, s)
	e.x += width
}
func (e *Editor) newline() {
	os.Stdout.Write([]byte{'\r', '\n'})
	e.x, e.y = 0, e.y+1
}
func (e *Editor) move(x, y int) {
	if y > e.y {
		fmt.Printf("\033[%dB", y-e.y)
	} else if y < e.y {
		fmt.Printf("\033[%dA", e.y-y)
	}
	if x > e.x {
		fmt.Printf("\033[%dC", x-e.x)
	} else if x < e.x {
		fmt.Printf("\033[%dD", e.x-x)
	}
	e.x, e.y = x, y
}

func autocomplete(s string, choices []string) (match string) {
	for _, c := range choices {
		if len(c) > len(s) && strings.EqualFold(s, c[:len(s)]) {
			if match == "" {
				match = c
			} else {
				match = c[:diffindex(c, match)]
			}
		}
	}
	return match
}
func prevword(s string, cur int) int {
	for cur > 0 && (s[cur] == ' ' || s[cur] == '\n') {
		cur--
	}
	for cur > 0 && s[cur-1] != ' ' && s[cur] != '\n' {
		cur--
	}
	return cur
}
func nextword(s string, cur int) int {
	l := len(s)
	for cur < l && (s[cur] == ' ' || s[cur] == '\n') {
		cur++
	}
	for cur < l && s[cur] != ' ' && s[cur] != '\n' {
		cur++
	}
	return cur
}
func diffindex(a, b string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) > len(b) {
		return len(b)
	}
	if len(b) > len(a) {
		return len(a)
	}
	return invalid
}
