//go:build !windows

package terminal

import (
	"os"

	"golang.org/x/term"
)

var isTerminal bool
var initialState *term.State

func init() {
	isTerminal = term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	if !isTerminal {
		return
	}
	var err error
	if initialState, err = term.GetState(int(os.Stdin.Fd())); err != nil {
		isTerminal = false
	}
}

func rawMode() {
	term.MakeRaw(int(os.Stdin.Fd()))
}

func cookedMode() {
	term.Restore(int(os.Stdin.Fd()), initialState)
}
