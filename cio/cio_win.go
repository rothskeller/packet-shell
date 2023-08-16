//go:build windows

package cio

import (
	"os"
	"strconv"

	"golang.org/x/term"
)

var (
	initialStateIn  uint32
	initialStateOut uint32
)

// Detect determines whether or not standard input and output are terminals,
// the screen width, and the initial state of the terminal.
func Detect() {
	// On Windows, it also engages output virtual terminal processing.
	var istate, ostate uint32

	err := windows.GetConsoleMode(windows.Handle(int(os.Stdin.Fd())), &istate)
	InputIsTerm = err == nil
	if InputIsTerm && initialStateIn == 0 {
		initialStateIn = istate
	}
	err = windows.GetConsoleMode(windows.Handle(int(os.Stdout.Fd())), &ostate)
	OutputIsTerm = err == nil
	if OutputIsTerm {
		if ostate&0x0005 != 0x0005 {
			ostate |= 0x0004 // ENABLE_VIRTUAL_TERMINAL_PROCESSING
			ostate |= 0x0001 // ENABLE_PROCESSED_OUTPUT
			windows.SetConsoleMode(windows.Handle(int(os.Stdout.Fd())), ostate)
		}
		if initialStateOut == 0 {
			initialStateOut = ostate
		}
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
	windows.SetConsoleMode(windows.Handle(int(os.Stdin.Fd())), 0x0200) // ENABLE_VIRTUAL_TERMINAL_INPUT
}

func restoreTerminal(st state) {
	windows.SetConsoleMode(windows.Handle(int(os.Stdin.Fd())), initialStateIn)
	windows.SetConsoleMode(windows.Handle(int(os.Stdout.Fd())), initialStateOut)
}
