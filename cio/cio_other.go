//go:build !windows

package cio

import (
	"os"
	"strconv"

	"golang.org/x/term"
)

var (
	initialStateIn  *term.State
	initialStateOut *term.State
)

// Detect determines whether or not standard input and output are terminals,
// the screen width, and the initial state of the terminal.
func Detect() {
	var istate, ostate *term.State
	var err error

	istate, err = term.GetState(int(os.Stdin.Fd()))
	InputIsTerm = err == nil
	if InputIsTerm && initialStateIn == nil {
		initialStateIn = istate
	}
	ostate, err = term.GetState(int(os.Stdout.Fd()))
	OutputIsTerm = err == nil
	if OutputIsTerm && initialStateOut == nil {
		initialStateOut = ostate
	}
	if OutputIsTerm {
		Width, _, _ = term.GetSize(int(os.Stdout.Fd()))
	}
	if Width == 0 {
		if w, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && w > 0 {
			Width = w
		} else {
			Width = 80
		}
	}
}

func rawMode() {
	term.MakeRaw(int(os.Stdin.Fd()))
}

func restoreTerminal() {
	term.Restore(int(os.Stdin.Fd()), initialStateIn)
}
