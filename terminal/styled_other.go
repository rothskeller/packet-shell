//go:build !windows

package terminal

import (
	"os"

	"golang.org/x/term"
)

type state *term.State

func (t *styled) makeRaw() (st state) {
	st, _ = term.MakeRaw(int(os.Stdin.Fd()))
	return st
}

func (t *styled) restore(st state) {
	term.Restore(int(os.Stdin.Fd()), st)
}
