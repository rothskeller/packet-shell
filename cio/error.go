package cio

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func Error(f string, args ...any) {
	var s = f
	if len(args) != 0 {
		s = fmt.Sprintf(f, args...)
	}
	if !strings.HasPrefix(s, "usage: ") {
		s = "ERROR: ⇥" + s
	}
	if OutputIsTerm {
		clearStatus()
		print(colorError, WrapText(s))
		setColor(0)
	} else {
		io.WriteString(os.Stderr, WrapText(s))
	}
}

func Confirm(f string, args ...any) {
	if OutputIsTerm {
		var s = f
		if len(args) != 0 {
			s = fmt.Sprintf(f, args...)
		}
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		clearStatus()
		io.WriteString(os.Stdout, s)
	} // else don't emit
}

func Welcome(f string, args ...any) {
	var s = f
	if len(args) != 0 {
		s = fmt.Sprintf(f, args...)
	}
	s = strings.TrimRight(s, "\n")
	if OutputIsTerm {
		print(colorLabel, s)
		print(0, "\n")
	} else {
		io.WriteString(os.Stderr, s)
		io.WriteString(os.Stderr, "\n")
	}
}
