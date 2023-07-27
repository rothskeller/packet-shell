// Package editfield provides an ANSI-terminal-based editor for a field value.
package editfield

import (
	"strings"
)

// Display displays a field label and value, in a manner congruent to how it
// looks while being edited.  This is used at the completion of an edit, to
// show the result after any post-formatting.  It is also used when displaying
// a set of fields without editing.
func (e *Editor) Display(label, value string) {
	var (
		labelWidth int
		linelen    int
		lines      []string
		indent     string
	)
	e.out(resetAndClearEOS, 0)
	e.out(fieldLabelStyle, 0)
	e.out(label, 0)
	e.out(normalStyle, 0)
	// Find the length of the longest line in the value.
	labelWidth = max(e.labelWidth, len(label))
	value = strings.TrimRight(value, "\n")
	lines = strings.Split(value, "\n")
	for _, line := range strings.Split(value, "\n") {
		linelen = max(linelen, len(line))
	}
	if linelen <= e.screenWidth-labelWidth-3 {
		e.out(spaces[:labelWidth+2-len(label)], 0)
		indent = spaces[:labelWidth+2]
	} else {
		lines, _ = wrap(value, e.screenWidth-5)
		e.newline()
		e.out(spaces[:4], 0)
		indent = spaces[:4]
	}
	for i, line := range lines {
		if i != 0 {
			e.out(indent, 0)
		}
		e.out(line, 0)
		e.newline()
	}
}

// DisplayError displays an error message (generally a problem with a field
// value).
func (e *Editor) DisplayError(err string) {
	lines, _ := wrap(err, e.screenWidth-1)
	for _, line := range lines {
		e.out(errorStyle, 0)
		e.out(line, 0)
		e.out(normalStyle, 0)
		e.newline()
	}
}
