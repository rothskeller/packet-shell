package main

import (
	"github.com/rothskeller/packet-cmd/shell"
	"github.com/rothskeller/packet/message/allmsg"
)

func main() {
	allmsg.Register()
	shell.Main()
}
