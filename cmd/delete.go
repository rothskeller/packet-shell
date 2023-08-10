package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rothskeller/packet-cmd/config"
	"github.com/rothskeller/packet/incident"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(deleteCmd)
}

var deleteCmd = &cobra.Command{
	Use:                   "delete «full-message-id»",
	DisableFlagsInUseLine: true,
	SuggestFor:            []string{"kill", "remove"},
	Short:                 "Delete an unsent message",
	Long: `The "delete" command deletes an outgoing message that has not yet been sent.
For safety, the argument must be the full local message ID of the message; the
usual shorthands for message IDs are not accepted by the "delete" command.
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		args[0] = strings.ToUpper(args[0])
		if !config.MsgIDRE.MatchString(args[0]) {
			return fmt.Errorf(`%q is not a valid, complete message ID`, args[0])
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
		term.Confirm("%s deleted.", args[0])
		return nil
	},
}
