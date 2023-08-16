package cmd

import "errors"

const quitSlug = `Quit the packet shell`
const quitHelp = `
usage: packet quit

The "quit" (or "q" or "exit") command quits the packet shell.
`

var ErrQuit = errors.New("quit requested")
