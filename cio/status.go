package cio

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var SuppressStatus bool

func Status(f string, args ...any) {
	if OutputIsTerm && !SuppressStatus {
		clearStatus()
		if f != "" {
			var s = f
			if len(args) != 0 {
				s = fmt.Sprintf(f, args...)
			}
			io.WriteString(os.Stdout, strings.TrimRight(s, "\n"))
			haveStatus = true
		}
	} // else print nothing
}
