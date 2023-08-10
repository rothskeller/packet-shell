//go:build windows

package terminal

import (
	"os"

	"golang.org/x/sys/windows"
)

var isTerminal bool
var initialState uint32

func init() {
	isTerminal = true
	if err := windows.GetConsoleMode(windows.Handle(int(os.Stdin.Fd())), &initialState); err != nil {
		isTerminal = false
		return
	}
	var ostate uint32
	if err := windows.GetConsoleMode(windows.Handle(int(os.Stdout.Fd())), &ostate); err != nil {
		isTerminal = false
		return
	}
	ostate |= 0x0004 // ENABLE_VIRTUAL_TERMINAL_PROCESSING
	ostate |= 0x0001 // ENABLE_PROCESSED_OUTPUT
	if err := windows.SetConsoleMode(windows.Handle(int(os.Stdout.Fd())), ostate); err != nil {
		isTerminal = false
	}
}

func rawMode() {
	if isTerminal {
		windows.SetConsoleMode(windows.Handle(int(os.Stdin.Fd())), 0x0200) // ENABLE_VIRTUAL_TERMINAL_INPUT
	}
}

func cookedMode() {
	windows.SetConsoleMode(windows.Handle(int(os.Stdin.Fd())), initialState)
}
