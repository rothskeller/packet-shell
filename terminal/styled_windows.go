//go:build windows

package terminal

import (
	"os"

	"golang.org/x/sys/windows"
)

type state uint64

func isTerminal() (isTerminal bool) {
	var dummy uint32

	if err := windows.GetConsoleMode(windows.Handle(int(os.Stdin.Fd())), &dummy); err != nil {
		return false
	}
	if err := windows.GetConsoleMode(windows.Handle(int(os.Stdout.Fd())), &dummy); err != nil {
		return false
	}
	return true
}

func openTerminal() (st state) {
	var istate, ostate uint32
	windows.GetConsoleMode(windows.Handle(int(os.Stdin.Fd())), &istate)
	windows.GetConsoleMode(windows.Handle(int(os.Stdout.Fd())), &ostate)
	ostate |= 0x0004 // ENABLE_VIRTUAL_TERMINAL_PROCESSING
	ostate |= 0x0001 // ENABLE_PROCESSED_OUTPUT
	windows.SetConsoleMode(windows.Handle(int(os.Stdout.Fd())), ostate)
	return istate<<32 | ostate
}

func rawMode() {
	windows.SetConsoleMode(windows.Handle(int(os.Stdin.Fd())), 0x0200) // ENABLE_VIRTUAL_TERMINAL_INPUT
}

func restoreTerminal(st state) {
	windows.SetConsoleMode(windows.Handle(int(os.Stdin.Fd())), uint32(st>>32))
	windows.SetConsoleMode(windows.Handle(int(os.Stdout.Fd())), uint32(st))
}
