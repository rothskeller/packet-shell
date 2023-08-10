package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(setCmd)
	setCmd.Flags().Bool("force", false, "allow invalid value")
}

var setCmd = &cobra.Command{
	Use:                   "set [--force] «message-id» «field-name» [«value»]",
	DisableFlagsInUseLine: true,
	Short:                 "Set the value of a field of a message",
	Long: `The "set" command sets the value of a field of a message.  If a «value» is
provided on the command line, it is used; otherwise, the new value is read
from standard input.  The provided value must be valid for the field unless
the --force flag is given.

«message-id» must be the local message ID of an unsent outgoing message.  It
can be just the numeric part of the ID if that is unique.

«field-name» is the name of the field to set.  It can be the PackItForms tag
for the field (including the trailing period, if any), or it can be the full
field name.  In interactive (--human) mode, it can be a shortened version of
the field name, such as "ocs" for "Operator Call Sign."
`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			lmi       string
			env       *envelope.Envelope
			msg       message.Message
			fields    []*message.Field
			field     *message.Field
			problems  map[*message.Field]string
			newprob   bool
			lmichange string
		)
		if lmi, err = expandMessageID(args[0], false); err != nil {
			return err
		}
		if env, msg, err = incident.ReadMessage(lmi); err != nil {
			return fmt.Errorf("reading %s: %s", lmi, err)
		}
		if env.IsReceived() {
			return errors.New("cannot set field of received message")
		}
		if env.IsFinal() {
			return errors.New("message has already been sent")
		}
		if !msg.Editable() {
			return fmt.Errorf("%ss are not editable", msg.Base().Type.Name)
		}
		// Build the list of fields that can be set.  This includes the
		// To address list, an all editable fields.  We disregard
		// EditSkip; that allows addressing fields with PIFOTags that
		// are normally aggregated into other fields and not directly
		// editable.
		fields = append(fields, newToAddressField(&env.To))
		for _, f := range msg.Base().Fields {
			if f.EditHelp != "" {
				fields = append(fields, f)
			}
		}
		// Verify that we have a valid field to edit.
		if field, err = expandFieldName(fields, args[1], term.Human()); err != nil {
			return err
		}
		// Find out what problems already exist in the message.
		problems = make(map[*message.Field]string)
		for _, f := range fields {
			problems[f] = f.EditValid(f)
		}
		// If we were given a new value on the command line, apply it.
		// Otherwise, allow the user to edit the field.
		if len(args) > 2 {
			field.EditApply(field, strings.Join(args[2:], " "))
		} else if _, err = term.EditField(field, 0); err != nil {
			return err
		}
		// If we edited the LMI, check it.  We have to have a valid one
		// to save the file.  If they changed it, make sure the new one
		// isn't already in use.
		if field.Value == msg.Base().FOriginMsgID {
			if p := field.EditValid(field); p != "" {
				return errors.New(p)
			}
			var newlmi = *field.Value
			if newlmi != lmi {
				if incident.UniqueMessageID(newlmi) != newlmi {
					return fmt.Errorf("message %s already exists", newlmi)
				}
				lmichange = newlmi
			}
		}
		// Report any new problems.
		for _, f := range fields {
			if p := f.EditValid(f); p != "" && (p != problems[f] || f == field) {
				term.Error(p)
				newprob = true
			}
		}
		if newprob {
			if force, _ := cmd.Flags().GetBool("force"); force {
				term.Confirm("NOTE: applying the changes anyway since --force was used")
				if env.ReadyToSend && fields[0].EditValid(fields[0]) != "" {
					env.ReadyToSend = false
					term.Confirm("NOTE: removing from send queue; can't send without valid To address")
				}
			} else {
				return errors.New("change not applied; use --force to override")
			}
		}
		// Apply the change.
		if lmichange != "" {
			if err = incident.SaveMessage(lmichange, "", env, msg); err != nil {
				return fmt.Errorf("saving %s: %s", lmichange, err)
			}
			incident.RemoveMessage(lmi)
			return nil
		}
		if err = incident.SaveMessage(lmi, "", env, msg); err != nil {
			return fmt.Errorf("saving %s: %s", lmi, err)
		}
		return nil
	},
}
