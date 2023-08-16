package cio

import (
	"encoding/csv"
	"io"
	"os"
	"strings"
)

var tableCW *csv.Writer

func ShowNameValue(name, value string, nameWidth int) {
	if OutputIsTerm {
		showNameValueTable(name, value, nameWidth)
	} else {
		showNameValueCSV(name, value, nameWidth != 0)
	}
}

func showNameValueCSV(name, value string, multiple bool) {
	if multiple {
		if tableCW == nil {
			tableCW = csv.NewWriter(os.Stdout)
		}
		tableCW.Write([]string{name, value})
	} else {
		io.WriteString(os.Stdout, value)
		io.WriteString(os.Stdout, "\n")
	}
}

func showNameValueTable(name, value string, nameWidth int) {
	var (
		linelen int
		lines   []string
		indent  string
	)
	clearStatus()
	// Find the length of the longest line in the value.
	nameWidth = max(nameWidth, len(name))
	value = strings.TrimRight(value, "\n")
	lines = strings.Split(value, "\n")
	for _, line := range strings.Split(value, "\n") {
		linelen = max(linelen, len(line))
	}
	// If the longest line fits to the right of the name, show it that way.
	// Otherwise, show it on the following lines with a 4-space indent.
	if linelen <= Width-nameWidth-3 {
		print(colorLabel, setLength(name, nameWidth)+"  ")
		indent = spaces[:nameWidth+2]
	} else {
		print(colorLabel, name)
		print(0, "\n    ")
		lines, _ = wrap(value, Width-5)
		indent = spaces[:4]
	}
	// Show the lines.
	for i, line := range lines {
		if i != 0 {
			print(0, indent)
		}
		print(0, line)
		print(0, "\n")
	}
}

func EndNameValueList() {
	if tableCW != nil {
		tableCW.Flush()
		tableCW = nil
	}
}
