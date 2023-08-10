//go:build windows

package terminal

import (
	"os"

	"golang.org/x/sys/windows"
)

type state uint64

func (t *styled) makeRaw() state {
	var ist, ost uint32
	if err := windows.GetConsoleMode(windows.Handle(int(os.Stdin.Fd())), &ist); err != nil {
		return 0
	}
	if err := windows.GetConsoleMode(windows.Handle(int(os.Stdout.Fd())), &ost); err != nil {
		return 0
	}
	const iraw = 0x0200 /* ENABLE_VIRTUAL_TERMINAL_INPUT */
	if err := windows.SetConsoleMode(windows.Handle(int(os.Stdin.Fd())), iraw); err != nil {
		return 0
	}
	const oraw = 0x0004 /* ENABLE_VIRTUAL_TERMINAL_PROCESSING */ | 0x0001 /* ENABLE_PROCESSED_OUTPUT */
	if err := windows.SetConsoleMode(windows.Handle(int(os.Stdout.Fd())), oraw); err != nil {
		return 0
	}
	return state(ist)<<32 + state(ost)
}

func (t *styled) restore(st state) {
	windows.SetConsoleMode(windows.Handle(int(os.Stdin.Fd())), uint32(st>>32))
	windows.SetConsoleMode(windows.Handle(int(os.Stdout.Fd())), uint32(st&0xFFFFFFFF))
}
