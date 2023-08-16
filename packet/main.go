package main

import (
	"os"

	"github.com/rothskeller/packet-shell/cmd"
	"github.com/rothskeller/packet/xscmsg"
)

func main() {
	xscmsg.Register()
	if !cmd.Run(os.Args[1:]) {
		os.Exit(1)
	}
}
