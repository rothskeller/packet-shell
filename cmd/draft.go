package cmd

import (
	"errors"
	"fmt"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/spf13/pflag"
)

const draftSlug = `Remove an unsent message from the send queue`
const draftHelp = `
usage: packet draft «message-id»

The "draft" command removes an unsent message from the send queue, returning it to draft status.  «message-id» must be the local message ID of an unsent outgoing message.  It can be just the numeric part of the ID if that is unique.
`

func cmdDraft(args []string) (err error) {
	var (
		lmi string
		env *envelope.Envelope
		msg message.Message
		li  *cio.ListItem
	)
	var flags = pflag.NewFlagSet("draft", pflag.ContinueOnError)
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"draft"})
	} else if err != nil {
		cio.Error(err.Error())
		return usage(draftHelp)
	}
	if len(args) != 1 {
		return usage(draftHelp)
	}
	if lmi, err = expandMessageID(args[0], false); err != nil {
		return err
	}
	if env, msg, err = incident.ReadMessage(lmi); err != nil {
		return fmt.Errorf("reading %s: %s", lmi, err)
	}
	if env.IsFinal() {
		if env.IsReceived() {
			return errors.New("message is a received message")
		} else {
			return errors.New("message has already been sent")
		}
	}
	if env.ReadyToSend {
		env.ReadyToSend = false
		if err = incident.SaveMessage(lmi, "", env, msg, false, false); err != nil {
			return fmt.Errorf("saving %s: %s", lmi, err)
		}
	}
	li = listItemForMessage(lmi, "", env)
	li.NoHeader = true
	cio.ListMessage(li)
	return nil
}
