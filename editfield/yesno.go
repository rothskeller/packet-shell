package editfield

// YesNo asks a yes-or-no question.  It returns true if the answer is yes, false
// for no or error.
func (e *Editor) YesNo(question string, value bool) bool {
	var yesx int

	// Draw the label.
	e.x, e.y = 0, 0
	e.out(resetAndClearEOS, 0)
	e.out(fieldLabelStyle, 0)
	e.out(question, len(question))
	e.out(normalStyle, 0)
	yesx = e.labelWidth + 2
	if len(question) > e.labelWidth {
		yesx = len(question) + 2
	}
	e.out(hideCursor, 0)
	defer func() {
		e.move(yesx, 0)
		if value {
			e.out("Yes", 3)
		} else {
			e.out("No", 2)
		}
		e.out(clearEOL, 0)
		e.newline()
		e.out(showCursor, 0)
	}()
	// Run the input loop.
	for {
		// Return the cursor to the start of the entry area.
		e.move(yesx, 0)
		if value {
			e.out(selectedStyle, 0)
			e.out("Yes", 3)
			e.out(normalStyle, 0)
		} else {
			e.out("Yes", 3)
		}
		e.out("  ", 2)
		if !value {
			e.out(selectedStyle, 0)
			e.out("No", 2)
			e.out(normalStyle, 0)
		} else {
			e.out("No", 2)
		}
		// Read a key and handle it.
		switch key := readKey(); key {
		case 0x00, 0x03, 0x1B:
			return false
		case 0x0A, 0x0D: // Enter
			return value
		case keyRight, keyLeft:
			value = !value
		case 'y', 'Y':
			return true
		case 'n', 'N':
			return false
		}
	}
}
