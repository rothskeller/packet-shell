package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet/incident"

	"github.com/spf13/pflag"
)

const (
	deleteSlug = `Delete an unsent message`
	deleteHelp = `
usage: packet delete «full-message-id»

The "delete" command deletes an outgoing message that has not yet been sent.  For safety, the argument must be the full local message ID of the message; the usual shorthands for message IDs are not accepted by the "delete" command.
`
)

func cmdDelete(args []string) (err error) {
	flags := pflag.NewFlagSet("delete", pflag.ContinueOnError)
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"delete"})
	} else if err != nil {
		cio.Error("%s", err.Error())
		return usage(deleteHelp)
	}
	if len(args) != 1 {
		return usage(deleteHelp)
	}
	args[0] = strings.ToUpper(args[0])
	if !incident.MsgIDRE.MatchString(args[0]) {
		cio.Error(`%q is not a valid, complete message ID`, args[0])
		return usage(deleteHelp)
	}
	env, _, err := incident.ReadMessage(args[0])
	if err != nil {
		return fmt.Errorf("read %s: %s", args[0], err)
	}
	if env.IsFinal() {
		if env.IsReceived() {
			return errors.New("can't delete a received message")
		} else {
			return errors.New("message has already been sent")
		}
	}
	incident.RemoveMessage(args[0])
	cio.Confirm("%s deleted.", args[0])
	return nil
}
