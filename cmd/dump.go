package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet-shell/config"
	"github.com/spf13/pflag"
)

const dumpSlug = `Show a message in encoded form`
const dumpHelp = `
usage: packet dump «message-id»

The "dump" command displays a message in its PackItForms- and RFC-5322-encoded format, as it would be transmitted over the air.  «message-id» must be the local or remote message ID of the message to display.  It can be just the numeric part of the ID if that is unique.
`

func cmdDump(args []string) (err error) {
	var (
		lmi string
		fh  *os.File
	)
	var flags = pflag.NewFlagSet("dump", pflag.ContinueOnError)
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"dump"})
	} else if err != nil {
		cio.Error(err.Error())
		return usage(dumpHelp)
	}
	if len(args) != 1 {
		return usage(dumpHelp)
	}
	if lmi, err = expandMessageID(args[0], true); err != nil {
		return err
	}
	if fh, err = os.Open(lmi + ".txt"); err != nil {
		return fmt.Errorf("reading %s: %s", lmi, err)
	}
	io.Copy(os.Stdout, fh)
	fh.Close()
	if config.C.Unread[lmi] {
		config.C.Unread[lmi] = false
		config.SaveConfig()
	}
	return nil
}
