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
	rootCmd.AddCommand(draftCmd)
}

var draftCmd = &cobra.Command{
	Use:                   "draft «message-id»",
	DisableFlagsInUseLine: true,
	SuggestFor:            []string{"unqueue", "dequeue"},
	Short:                 "Remove an unsent message from the send queue",
	Long: `The "draft" command removes an unsent message from the send queue, returning
it to draft status.

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
		if env.ReadyToSend {
			env.ReadyToSend = false
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
