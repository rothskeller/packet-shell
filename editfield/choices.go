package editfield

// choicesMode handles field editing in choices mode.  We get in here if the
// field has choices, and is either empty or its value is one of the choices.
func (e *Editor) choicesMode() (modefunc, Result) {
	var (
		xs      []int
		choices [][]string
		entryx  int
		selline int
		selcol  int
	)
	// Compute the layout for the choices.
	for lines := 1; xs == nil; lines = lines + 1 {
		xs, choices = e.choicesLayout(lines)
	}
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
	// Draw each line of choices.
	for lnum, line := range choices {
		for lnum > e.y {
			e.newline()
		}
		for cnum, choice := range line {
			x := xs[cnum]
			if choice == "" {
				x = entryx
			}
			e.out(spaces[:x-e.x], 0)
			e.x = x
			if choice == "" {
				e.out(entryStyle, 0)
				e.out(spaces[:4], 4)
				e.out(normalStyle, 0)
			} else if e.value == choice {
				selline, selcol = lnum, cnum
				e.out(selectedStyle, 0)
				e.out(choice, len(choice))
				e.out(normalStyle, 0)
			} else {
				e.out(choice, len(choice))
			}
		}
	}
	// Run the input loop.
	for {
		var newline, newcol = selline, selcol

		// Return the cursor to the start of the entry area.
		e.move(entryx, 0)
		if e.value != "" {
			e.out(hideCursor, 0)
		} else {
			e.out(showCursor, 0)
		}
		// Read a key and handle it.
		switch key := readKey(); key {
		case 0:
			return nil, ResultError
		case 0x03: // Ctrl-C
			return nil, ResultCtrlC
		case 0x08, 0x7f, keyDelete:
			newline, newcol = 0, 0
		case 0x09: // Tab
			return nil, ResultTab
		case keyBackTab:
			return nil, ResultBackTab
		case 0x0A, 0x0D: // Enter
			return nil, ResultEnter
		case 0x0C: // Ctrl-L
			return e.choicesMode, 0
		case 0x1B:
			return nil, ResultESC
		case keyUp:
			newline--
		case keyDown:
			newline++
		case keyRight:
			newcol++
		case keyLeft:
			newcol--
		case keyHome:
			newcol = 0
		case keyEnd:
			newcol = len(xs)
		case keyF1:
			e.showHelp()
			return e.choicesMode, 0
		default:
			if key >= 0x20 && key <= 0x7e && e.value == "" {
				// Printable character, switch to oneline mode.
				unreadKey(key)
				return e.onelineMode, 0
			}
		}
		// Make sure the new selection is in range.
		if newline < 0 {
			newline = 0
		}
		if newline >= len(choices) {
			newline = len(choices) - 1
		}
		if newcol < 0 {
			newcol = 0
		}
		if newcol >= len(choices[newline]) {
			newcol = len(choices[newline]) - 1
		}
		// If nothing changed, get another keystroke.
		if newline == selline && newcol == selcol {
			continue
		}
		e.changed = true
		// If we previously had a selection, redraw it to be unselected.
		if selline != 0 || selcol != 0 {
			e.move(xs[selcol], selline)
			e.out(e.value, len(e.value))
		}
		// Apply the new selection.
		selline, selcol, e.value = newline, newcol, choices[newline][newcol]
		// If we have a new selection, redraw it to appear selected.
		if selline != 0 || selcol != 0 {
			e.move(xs[selcol], selline)
			e.out(selectedStyle, 0)
			e.out(e.value, len(e.value))
			e.out(normalStyle, 0)
		}
	}
}

// choicesLayout computes the layout for the choices, assuming the supplied line
// count.  It returns two slices.  The first is a slice giving the X coordinate
// of the start of each column of choices.  The second is a slice of lines, each
// of which contains a slice of choices on that line.  If the choices will not
// fit in the specified number of lines, choicesLayout returns nil, nil.
//
// Note that the layout includes the empty choice on line 0, followed by
// linecount additional lines with other choices.  The exception is when all
// choices fit on line 0 after the empty choice; in that case, a single line is
// returned.
func (e *Editor) choicesLayout(linecount int) (xs []int, choices [][]string) {
	choices = make([][]string, linecount+1)
	choices[0] = []string{""} // the empty entry area
	xs = []int{4}
	var x = 4
	var labelWidth = e.labelWidth
	if len(e.label) > labelWidth {
		labelWidth = len(e.label)
	}
	// Place each of the choices.
	for i, c := range e.choices {
		if i != 0 && i%linecount == 0 {
			// Finished filling a column.  Set for the next one.
			x += 2
			xs = append(xs, x)
		}
		choices[i%linecount+1] = append(choices[i%linecount+1], c)
		if newx := xs[i/linecount] + len(c); newx > x {
			x = newx
		}
		if x >= e.screenWidth {
			return nil, nil // can't fit with this linecount
		}
	}
	// If we have only one line and it can fit on the entry area line,
	// adjust for that.
	if linecount == 1 && x+labelWidth+4 < e.screenWidth {
		for i := range xs {
			xs[i] += labelWidth + 4
		}
		xs = append([]int{labelWidth + 2}, xs...)
		choices = [][]string{append(choices[0], choices[1]...)}
	}
	return xs, choices
}
