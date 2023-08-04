package main

import (
	"github.com/rothskeller/packet-cmd/shell"
	"github.com/rothskeller/packet/xscmsg/allmsg"
)

func main() {
	allmsg.Register()
	shell.Main()
}
