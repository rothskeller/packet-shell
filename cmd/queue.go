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

const queueSlug = `Add an unsent message to the send queue`
const queueHelp = `
usage: packet queue [--force] «message-id»
  --force  queue a message with invalid contents

The "queue" command adds an unsent message to the send queue, if it is not already there.  The message will be sent during the next BBS connection.  «message-id» must be the local message ID of an unsent outgoing message.  It can be just the numeric part of the ID if that is unique.
`

func cmdQueue(args []string) (err error) {
	var (
		force bool
		lmi   string
		env   *envelope.Envelope
		msg   message.Message
		li    *cio.ListItem
		flags = pflag.NewFlagSet("queue", pflag.ContinueOnError)
	)
	flags.BoolVar(&force, "force", false, "queue a message with invalid contents")
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"queue"})
	} else if err != nil {
		cio.Error(err.Error())
		return usage(queueHelp)
	}
	args = flags.Args()
	if len(args) != 1 {
		return usage(queueHelp)
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
	if env.To == "" {
		return errors.New("message cannot be queued without a To: address")
	}
	if !env.ReadyToSend {
		if !force {
			if problems := msg.PIFOValid(); len(problems) != 0 {
				for _, p := range problems {
					cio.Error(p)
				}
				return errors.New("not queueing because message is invalid and --force was not used")
			}
		}
		env.ReadyToSend = true
		if err = incident.SaveMessage(lmi, "", env, msg, false); err != nil {
			return fmt.Errorf("saving %s: %s", lmi, err)
		}
	}
	li = listItemForMessage(lmi, "", env)
	li.NoHeader = true
	cio.ListMessage(li)
	return nil
}
