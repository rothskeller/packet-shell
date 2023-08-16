package cio

import (
	"bufio"
	"errors"
	"io"
	"os"
)

var history []string

func ReadCommand() (line string, err error) {
	if InputIsTerm {
		return readCommandTerminal()
	}
	return readCommandStdin()
}

func readCommandTerminal() (line string, err error) {
	var (
		cursor       int
		scroll       int
		selstart     int
		selend       int
		historyIndex = len(history)
	)
	history = append(history, "")
	rawMode()
	cleanTerminal()
	defer func() {
		move(0, 0)
		cleanTerminal()
		restoreTerminal()
	}()
	for {
		var buf = newScreenBuf(Width - 1)
		buf.writeAt(0, 0, colorLabel, "packet>")
		pre, sel, post := splitOnSelect(line[scroll:], selstart-scroll, selend-scroll)
		buf.writeAt(8, 0, 0, pre)
		buf.write(colorSelected, sel)
		buf.write(0, post)
		paintBuf(buf)
		move(8+cursor-scroll, 0)
		switch key := readKey(); key {
		case 0:
			return "", errors.New("error reading stdin")
		case 0x01, keyHome: // Ctrl-A
			cursor, selstart, selend = 0, 0, 0
		case keyShiftHome:
			cursor, selstart = 0, 0
		case 0x02, keyLeft: // Ctrl-B
			if cursor > 0 {
				cursor--
			}
			selstart, selend = cursor, cursor
		case keyShiftLeft:
			if selstart != selend && selend == cursor {
				selend, cursor = selstart-1, cursor-1
			} else if cursor > 0 {
				selstart, cursor = selstart-1, cursor-1
			}
		case keyCtrlLeft:
			cursor = prevword(line, cursor)
			selstart, selend = cursor, cursor
		case keyCtrlShiftLeft:
			if selstart != selend && selend == cursor {
				cursor = prevword(line, cursor)
				selend = cursor
				if selend < selstart {
					selstart = selend
				}
			} else {
				cursor = prevword(line, cursor)
				selstart = cursor
			}
		case 0x03: // Ctrl-C
			return "", errors.New("interrupted")
		case 0x04, keyDelete: // Ctrl-D
			if len(line) > cursor {
				line = line[:cursor] + line[cursor+1:]
			}
			selstart, selend = cursor, cursor
		case 0x05, keyEnd: // Ctrl-E, End
			cursor = len(line)
			selstart, selend = cursor, cursor
		case keyShiftEnd:
			cursor = len(line)
			selend = cursor
		case 0x06, keyRight: // Ctrl-F
			if cursor < len(line) {
				cursor++
			}
			selstart, selend = cursor, cursor
		case keyShiftRight:
			if selstart != selend && selstart == cursor {
				selstart, cursor = selstart+1, cursor+1
			} else if cursor < len(line) {
				selend, cursor = selend+1, cursor+1
			}
		case keyCtrlRight:
			cursor = nextword(line, cursor)
			selstart, selend = cursor, cursor
		case keyCtrlShiftRight:
			if selstart != selend && selstart == cursor {
				cursor = nextword(line, cursor)
				selstart = cursor
				if selstart < selend {
					selend = selstart
				}
			} else {
				cursor = nextword(line, cursor)
				selend = cursor
			}
		case 0x08, 0x7f: // Backspace
			if selstart != selend {
				line = line[:selstart] + line[selend:]
				cursor = selstart
			} else if cursor > 0 {
				line = line[:cursor-1] + line[cursor:]
				cursor--
			}
			selstart, selend = cursor, cursor
		case 0x0A, 0x0D: // Enter
			history[len(history)-1] = line
			move(8, 0)
			clearToEOL()
			print(0, line) // might wrap, but that's OK
			print(0, "\n")
			cleanTerminal()
			return line, nil
		case 0x0B: // Ctrl-K
			line = line[:cursor]
			selstart, selend = cursor, cursor
		case 0x0E, keyDown: // Ctrl-N
			if historyIndex < len(history)-1 {
				historyIndex++
				line = history[historyIndex]
				cursor = len(line)
				selstart, selend = cursor, cursor
			}
		case 0x10, keyUp:
			if historyIndex > 0 {
				if historyIndex == len(history)-1 {
					history[historyIndex] = line
				}
				historyIndex--
				line = history[historyIndex]
				cursor = len(line)
				selstart, selend = cursor, cursor
			}
		case 0x15: // Ctrl-U
			line = ""
			selstart, selend, cursor = 0, 0, 0
		default:
			if key >= 0x20 && key <= 0x7e { // Printable character
				line = line[:selstart] + string(key) + line[selend:]
				cursor = selstart + 1
				selstart, selend = cursor, cursor
			}
		}
		// Change the scrolling if needed to keep the cursor in view.
		scroll = min(scroll, cursor)
		scroll = max(scroll, cursor-Width+9)
	}
}

func readCommandStdin() (string, error) {
	var scan = bufio.NewScanner(os.Stdin)

	io.WriteString(os.Stdout, "packet> ")
	if scan.Scan() {
		return scan.Text(), nil
	}
	if err := scan.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}
