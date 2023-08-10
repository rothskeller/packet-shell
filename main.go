package main

import (
	"fmt"
	"os"

	"github.com/rothskeller/packet-cmd/cmd"
	"github.com/rothskeller/packet/xscmsg"
)

func main() {
	xscmsg.Register()
	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}
