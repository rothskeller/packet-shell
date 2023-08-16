package cio

import (
	"strings"
)

func (e *editor) display() {
	var (
		linelen int
		lines   []string
		indent  string
	)
	print(colorLabel, e.field.Label)
	// Find the length of the longest line in the value.
	lines = strings.Split(strings.TrimRight(e.value, "\n"), "\n")
	for _, line := range lines {
		linelen = max(linelen, len(line))
	}
	if linelen <= Width-e.labelWidth-3 {
		print(0, spaces[:e.labelWidth+2-len(e.field.Label)])
		indent = spaces[:e.labelWidth+2]
	} else {
		lines, _ = wrap(strings.TrimRight(e.value, "\n"), Width-5)
		print(0, "\n    ")
		indent = spaces[:4]
	}
	for i, line := range lines {
		if i != 0 {
			print(0, indent)
		}
		print(0, line)
		print(0, "\n")
	}
}
