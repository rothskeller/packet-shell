package editfield

import (
	"os"
)

// Key codes used in this program.  This isn't all possible key codes, but it's
// the ones that are relevant to us.
const (
	keyUp = 0x80 + iota
	keyDown
	keyRight
	keyLeft
	keyShiftUp
	keyShiftDown
	keyShiftRight
	keyShiftLeft
	keyCtrlRight
	keyCtrlLeft
	keyCtrlShiftRight
	keyCtrlShiftLeft
	keyHome
	keyEnd
	keyShiftHome
	keyShiftEnd
	keyDelete
	keyF1
	keyBackTab

	keyCtrlC = 0x03
)

var readKeyBuf [256]byte
var pendingKeys []byte

// readKeys reads input from the keyboard and returns it as a byte representing
// a single key.  ASCII printable characters have their defined values, and
// other keys of interest have values with the high bit set.  Since our data can
// only allow plain ASCII, this is adequate.  A zero return indicates a read
// error.
func readKey() (key byte) {
	for len(pendingKeys) == 0 {
		count, err := os.Stdin.Read(readKeyBuf[:])
		if err != nil || count == 0 {
			return 0
		}
		buf := readKeyBuf[:count]
		for len(buf) != 0 {
			if key, buf = extractKey(buf); key != 0 {
				pendingKeys = append(pendingKeys, key)
			}
		}
	}
	key, pendingKeys = pendingKeys[0], pendingKeys[1:]
	return key
}

// unreadKey returns a key to the input buffer.
func unreadKey(key byte) {
	pendingKeys = append([]byte{key}, pendingKeys...)
}

// extractKey extracts a key out of the input buffer, returning the key and the
// modified buffer.  It returns 0 for a key sequence we don't care about.
func extractKey(buf []byte) (key byte, _ []byte) {
	var ignore bool
	var p1, p2 int

	if buf[0] >= 0x80 {
		return 0, buf[1:] // ignore all high bit characters
	}
	if buf[0] != 0x1b {
		return buf[0], buf[1:] // everything except ESC returned as is
	}
	if buf = buf[1:]; len(buf) == 0 {
		return 0x1b, nil // ESC returned as is if nothing after it
	}
	if buf[0] == 0x1b {
		ignore, buf = true, buf[1:] // ESC meaning "meta" before escape sequence
	}
	for len(buf) != 0 && buf[0] >= 0x20 && buf[0] <= 0x2f {
		buf = buf[1:] // skip so-called "intermediate" bytes
		// (never seen these in practice, but the spec allows them)
	}
	if len(buf) == 0 {
		return 0, nil
	}
	if buf[0] == 'P' || buf[0] == 'X' || buf[0] == '^' || buf[0] == '_' {
		// DCS, SOS, PM, APC: skip until ST
		for len(buf) != 0 && (buf[0] != 0x1b || (len(buf) != 1 && buf[1] != 0x5c)) {
			buf = buf[1:]
		}
		if len(buf) > 2 {
			return 0, buf[2:]
		}
		return 0, nil
	}
	if buf[0] == ']' {
		// OSC: skip until BEL or ST
		for len(buf) != 0 && buf[0] != 0x07 && (buf[0] != 0x1b || (len(buf) != 1 && buf[1] != 0x5c)) {
			buf = buf[1:]
		}
		if buf[0] == 0x1b && len(buf) > 2 {
			return 0, buf[2:]
		}
		return 0, buf[1:]
	}
	if buf[0] == 'O' && len(buf) > 1 {
		switch buf[1] {
		case 'A':
			key = keyUp
		case 'B':
			key = keyDown
		case 'C':
			key = keyRight
		case 'D':
			key = keyLeft
		case 'H':
			key = keyHome
		case 'P':
			key = keyF1
		}
		if ignore {
			key = 0
		}
		return key, buf[2:]
	}
	if buf[0] != '[' {
		return 0, buf[1:] // not sure what it is
	}
	// We have a CSI escape sequence (the most common kind).  Read the
	// numeric parameters first, if any.
	buf = buf[1:]
NUMERIC:
	for len(buf) != 0 {
		switch buf[0] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			p2 = p2*10 + int(buf[0]-'0')
			buf = buf[1:]
		case ':', ';':
			p1, p2 = p2, 0
			buf = buf[1:]
		case '<', '=', '>', '?':
			// allowed by spec, but not used in any sequence we want
			ignore, buf = true, buf[1:]
		default:
			break NUMERIC
		}
	}
	// The spec allows for any number of "intermediate bytes" here, which
	// don't occur in any sequence we want.
	for len(buf) != 0 && buf[0] >= 0x20 && buf[0] <= 0x2f {
		ignore, buf = true, buf[1:]
	}
	if len(buf) == 0 {
		return 0, nil
	}
	// Switch based on the final character.
	switch buf[0] {
	case 'A':
		switch p2 {
		case 0, 1:
			key = keyUp
		case 2:
			key = keyShiftUp
		}
	case 'B':
		switch p2 {
		case 0, 1:
			key = keyDown
		case 2:
			key = keyShiftDown
		}
	case 'C':
		switch p2 {
		case 0, 1:
			key = keyRight
		case 2:
			key = keyShiftRight
		}
	case 'D':
		switch p2 {
		case 0, 1:
			key = keyLeft
		case 2:
			key = keyShiftLeft
		}
	case 'F':
		switch p2 {
		case 0, 1:
			key = keyEnd
		case 2:
			key = keyShiftEnd
		}
	case 'H':
		switch p2 {
		case 0, 1:
			key = keyHome
		case 2:
			key = keyShiftHome
		}
	case 'P':
		if p2 == 0 || p2 == 1 {
			key = keyF1
		}
	case 'Z':
		if p2 == 0 || p2 == 1 {
			key = keyBackTab
		}
	case '~':
		if p1 == 0 {
			p1, p2 = p2, 1
		}
		switch p1 {
		case 1, 7: // home
			switch p2 {
			case 0, 1:
				key = keyHome
			case 2:
				key = keyShiftHome
			}
		case 3: // delete
			if p2 == 0 || p2 == 1 {
				key = keyDelete
			}
		case 4, 8: // end
			switch p2 {
			case 0, 1:
				key = keyEnd
			case 2:
				key = keyShiftEnd
			}
		case 11: // F1
			if p2 == 0 || p2 == 1 {
				key = keyF1
			}
		}
	}
	if ignore {
		key = 0
	}
	return key, buf[1:]
}
