package terminal

import (
	"strings"
)

func (e *editor) display() {
	var (
		linelen int
		lines   []string
		indent  string
	)
	e.term.print(colorLabel, e.field.Label)
	// Find the length of the longest line in the value.
	lines = strings.Split(strings.TrimRight(e.value, "\n"), "\n")
	for _, line := range lines {
		linelen = max(linelen, len(line))
	}
	if linelen <= e.term.width-e.labelWidth-3 {
		e.term.print(0, spaces[:e.labelWidth+2-len(e.field.Label)])
		indent = spaces[:e.labelWidth+2]
	} else {
		lines, _ = wrap(strings.TrimRight(e.value, "\n"), e.term.width-5)
		e.term.print(0, "\n    ")
		indent = spaces[:4]
	}
	for i, line := range lines {
		if i != 0 {
			e.term.print(0, indent)
		}
		e.term.print(0, line)
		e.term.print(0, "\n")
	}
}
