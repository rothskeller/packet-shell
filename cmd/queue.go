package cmd

import (
	"errors"
	"fmt"

	"github.com/rothskeller/packet-cmd/terminal"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(queueCmd)
}

var queueCmd = &cobra.Command{
	Use:                   "queue «message-id»",
	DisableFlagsInUseLine: true,
	Short:                 "Add an unsent message to the send queue",
	Long: `
The "queue" command adds an unsent message to the send queue, if it is not
already there.  The message will be sent during the next BBS connection.

«message-id» must be the local message ID of an unsent outgoing message.  It
can be just the numeric part of the ID if that is unique.
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			lmi string
			env *envelope.Envelope
			msg message.Message
			li  *terminal.ListItem
		)
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
		if len(env.To) == 0 {
			return errors.New("message cannot be queued without a To: address")
		}
		if !env.ReadyToSend {
			env.ReadyToSend = true
			if err = incident.SaveMessage(lmi, "", env, msg); err != nil {
				return fmt.Errorf("saving %s: %s", lmi, err)
			}
		}
		li = listItemForMessage(lmi, "", env)
		li.NoHeader = true
		term.ListMessage(li)
		return nil
	},
}
