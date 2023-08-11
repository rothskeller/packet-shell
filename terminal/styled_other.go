//go:build !windows

package terminal

import (
	"os"

	"golang.org/x/term"
)

type state *term.State

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func openTerminal() (st state) {
	st, _ = term.GetState(int(os.Stdin.Fd()))
	return st
}

func rawMode() {
	term.MakeRaw(int(os.Stdin.Fd()))
}

func restoreTerminal(st state) {
	term.Restore(int(os.Stdin.Fd()), st)
}
