package main

import (
	"github.com/rothskeller/packet-cmd/shell"
	"github.com/rothskeller/packet/xscmsg"
)

func main() {
	xscmsg.Register()
	shell.Main()
}
