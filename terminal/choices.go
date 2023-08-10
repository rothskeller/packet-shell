package terminal

import "errors"

// choicesMode handles field editing in choices mode.  We get in here if the
// field has choices, and is either empty or its value is one of the choices.
func (e *editor) choicesMode() (modefunc, EditResult, error) {
	var (
		xs      []int
		choices [][]string
		entryx  int
		selline int
		selcol  int
	)
	e.term.clearToEOS()
	defer func() {
		e.term.move(0, 0)
		e.term.clearToEOS()
	}()
	// Compute the layout for the choices.
	for lines := 1; xs == nil; lines = lines + 1 {
		xs, choices = e.choicesLayout(lines)
	}
	// Draw the label.
	e.term.print(colorLabel, setLength(e.field.Label, e.labelWidth+2))
	entryx = e.term.x
	// Draw each line of choices.
	for lnum, line := range choices {
		for lnum > e.term.y {
			e.term.print(0, "\n")
		}
		for cnum, choice := range line {
			x := xs[cnum]
			if choice == "" {
				x = entryx
			}
			e.term.print(0, spaces[:x-e.term.x])
			if choice == "" {
				e.term.print(colorEntry, spaces[:4])
			} else if e.value == choice {
				selline, selcol = lnum, cnum
				e.term.print(colorSelected, choice)
			} else {
				e.term.print(0, choice)
			}
		}
	}
	// Run the input loop.
	for {
		var newline, newcol = selline, selcol

		// Return the cursor to the start of the entry area.
		e.term.move(entryx, 0)
		e.term.showCursor(e.value == "")
		// Read a key and handle it.
		switch key := e.term.readKey(); key {
		case 0:
			return nil, 0, errors.New("error reading stdin")
		case 0x03: // Ctrl-C
			return nil, 0, errors.New("interrupted")
		case 0x08, 0x7f, keyDelete:
			newline, newcol = 0, 0
		case 0x09: // Tab
			return nil, ResultNext, nil
		case keyBackTab:
			return nil, ResultPrevious, nil
		case 0x0A, 0x0D: // Enter
			return nil, ResultNext, nil
		case 0x0C: // Ctrl-L
			return e.choicesMode, 0, nil
		case 0x1B:
			return nil, ResultDone, nil
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
			return e.choicesMode, 0, nil
		default:
			if key >= 0x20 && key <= 0x7e && e.value == "" {
				// Printable character, switch to oneline mode.
				e.term.unreadKey(key)
				return e.onelineMode, 0, nil
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
			e.term.move(xs[selcol], selline)
			e.term.print(0, e.value)
		}
		// Apply the new selection.
		selline, selcol, e.value = newline, newcol, choices[newline][newcol]
		e.sels, e.sele = len(e.value), len(e.value)
		// If we have a new selection, redraw it to appear selected.
		if selline != 0 || selcol != 0 {
			e.term.move(xs[selcol], selline)
			e.term.print(colorSelected, e.value)
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
func (e *editor) choicesLayout(linecount int) (xs []int, choices [][]string) {
	choices = make([][]string, linecount+1)
	choices[0] = []string{""} // the empty entry area
	xs = []int{4}
	var x = 4
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
		if x >= e.term.width {
			return nil, nil // can't fit with this linecount
		}
	}
	// If we have only one line and it can fit on the entry area line,
	// adjust for that.
	if linecount == 1 && x+e.labelWidth+4 < e.term.width {
		for i := range xs {
			xs[i] += e.labelWidth + 4
		}
		xs = append([]int{e.labelWidth + 2}, xs...)
		choices = [][]string{append(choices[0], choices[1]...)}
	}
	return xs, choices
}
