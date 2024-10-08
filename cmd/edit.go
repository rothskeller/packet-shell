package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet-shell/config"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/spf13/pflag"
	"golang.org/x/exp/slices"
)

const editSlug = `Edit an unsent message`
const editHelp = `
usage: packet edit ⇥[flags] «message-id»|config [«field-name»]
  -e, --errors  ⇥edit only fields that have errors

The "edit" (or "e") command edits an unsent message.  It presents each field in turn and allows that field's value to be changed.  Note that the "edit" command cannot be used in scripted mode (see "packet help script").

«message-id» must be the local message ID of an unsent outgoing message.  It can be just the numeric part of the message ID if that is unique.  If the word "config" (or an abbreviation) is given instead, the "edit" command edits the incident / activation settings instead (see "packet help config").

The "edit" command normally starts with the first field of the message (or the first that has an error, if --errors is used).  If a «field-name» is specified, editing begins with that field instead.  «field-name» can be the PackItForms tag for the field (including the trailing period, if any), or it can be the full field name or a shortened version of the field name, such as "ocs" for "Operator Call Sign."

Usage of the editor depends on the capabilities of the standard output device (e.g., the terminal).  If it is fully capable, the following keys can be used:
    Ctrl-A, Home    ⇥move cursor to beginning of line (*)
    Ctrl-B, ←       ⇥move cursor to the left (*)(+)
    Ctrl-C          ⇥abort the edit and do not save any changes
    Ctrl-D, Delete  ⇥delete the character under the cursor
    Ctrl-E, End     ⇥move cursor to end of line (*)
    Ctrl-F, →       ⇥move cursor to the right (*)(+)
    Ctrl-H, Backsp  ⇥delete the character before the cursor
    Ctrl-I, Tab     ⇥save this field and move to the next field
    Shift-Tab       ⇥save this field and move to the previous field
    Ctrl-K          ⇥delete the remainder of the current line
    Ctrl-L          ⇥redraw the editor (in case of screen corruption)
    Ctrl-M, Enter   ⇥multi-line fields:  enter a newline
                    ⇥single-line fields: save field and move to next field
    Ctrl-N, ↓       ⇥move cursor down one line (*)
    Ctrl-P, ↑       ⇥move cursor up one line (*)
    Ctrl-U          ⇥delete the entire contents of the field
    Ctrl-V + Enter  ⇥enter a newline in a normally single-line field
    ESC             ⇥save this field and exit the editor
    F1              ⇥display online help for the field
    (*) with Shift, extend the selection in the direction of movement
    (+) arrows with Ctrl move by words instead of characters
Some fields have a discrete set of possible or recommended values.  For those fields, the editor will show the set of values and allow you to select from among them using the arrow keys.  Or, if you prefer to type, the editor will autocomplete your entry from that set.

If you enter an invalid value for a field, an appropriate error will be shown and you will be asked to enter that field again.  If you hit Enter on the value to confirm it, the value will be kept despite the error.

When finished editing, if the message is fully valid and not already queued to be sent, the editor will ask whether to queue it.  If the message is already queued but is not valid, it will be removed from the queue.  (To force the queueing of an invalid message, use the "packet queue" command.)
`

func cmdEdit(args []string) (err error) {
	var (
		errorsOnly bool
		lmi        string
		env        *envelope.Envelope
		msg        message.Message
		fieldname  string
		flags      = pflag.NewFlagSet("edit", pflag.ContinueOnError)
	)
	if !cio.InputIsTerm || !cio.OutputIsTerm {
		return errors.New("editing is supported only when stdin/stdout is a terminal")
	}
	flags.BoolVarP(&errorsOnly, "errors", "e", false, "edit only fields that have errors")
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"edit"})
	} else if err != nil {
		cio.Error(err.Error())
		return usage(editHelp)
	}
	if flags.NArg() < 1 || flags.NArg() > 2 {
		return usage(editHelp)
	}
	args = flags.Args()
	if strings.HasPrefix("config", args[0]) {
		lmi = "config"
		env = new(envelope.Envelope)
		msg = &config.C
	} else {
		if lmi, err = expandMessageID(args[0], false); err != nil {
			return err
		}
		if env, msg, err = incident.ReadMessage(lmi); err != nil {
			return fmt.Errorf("reading %s: %s", lmi, err)
		}
		if env.IsReceived() {
			return errors.New("can't edit a received message")
		}
		if env.IsFinal() {
			return errors.New("message has already been sent")
		}
		if !msg.Editable() {
			return fmt.Errorf("%ss do not support editing", msg.Base().Type.Name)
		}
	}
	if len(args) > 1 {
		fieldname = args[1]
	}
	return doEdit(lmi, env, msg, fieldname, errorsOnly)
}

// doEdit is the common code between edit, new, reply, and resend.
func doEdit(lmi string, env *envelope.Envelope, msg message.Message, startField string, errorsOnly bool) (err error) {
	var (
		fields     []*message.Field
		field      *message.Field
		saveConfig bool
		labelWidth = 18 // "Queue for Sending?"
		wasQueued  = env.ReadyToSend
	)
	// Build the list of fields to be edited.
	if lmi != "config" {
		fields = append(fields, newToAddressField(&env.To))
	}
	for _, f := range msg.Base().Fields {
		if f.EditHelp != "" {
			fields = append(fields, f)
			labelWidth = max(labelWidth, len(f.Label))
		}
	}
	// Determine the starting field.
	if field, err = expandFieldName(fields, startField, true); err != nil {
		return err
	}
	if lmi != "config" {
		fields = append(fields, newSendQueueField(fields, &env.ReadyToSend))
	}
	if startField == "" && !errorsOnly {
		cio.StartEdit()
	}
LOOP: // Run the editor loop.
	for {
		var result cio.EditResult

		if result, err = cio.EditField(field, labelWidth); err != nil {
			return err
		}
		switch result {
		case cio.ResultDone:
			break LOOP
		case cio.ResultNext:
			idx := slices.Index(fields, field) + 1
			for idx < len(fields) {
				field = fields[idx]
				if !field.EditSkip(field) && (!errorsOnly || field.EditValid(field) == "") {
					break
				}
				idx++
			}
			if idx >= len(fields) {
				break LOOP
			}
		case cio.ResultPrevious:
			idx := slices.Index(fields, field) - 1
			for idx >= 0 {
				field = fields[idx]
				if !field.EditSkip(field) && (!errorsOnly || field.EditValid(field) == "") {
					break
				}
				idx--
			}
			if idx < 0 {
				break LOOP
			}
		default:
			panic("unknown result code")
		}
	}
	// If editing the configuration, save it.
	if lmi == "config" {
		config.SaveConfig()
		return nil
	}
	// Make sure we have a valid LMI.  We have to have one to save the file.
	var newlmi string
	if omi := msg.Base().FOriginMsgID; omi != nil {
		newlmi = *omi
	}
	if !incident.MsgIDRE.MatchString(newlmi) {
		if lmi != "" {
			newlmi = lmi // restore the one it had when we started
		} else {
			newlmi = incident.UniqueMessageID("AAA-001P")
		}
		if omi := msg.Base().FOriginMsgID; omi != nil {
			*msg.Base().FOriginMsgID = newlmi
		}
		cio.Confirm("NOTE: The local message ID has been set to %s.", newlmi)
	}
	// Notify the user if we took the message out of the queue.
	if wasQueued && !env.ReadyToSend {
		cio.Confirm("NOTE: This message has invalid fields and has been removed from the send queue.")
	}
	// Check for a change to the LMI.
	if newlmi != lmi {
		if unique := incident.UniqueMessageID(newlmi); unique != newlmi {
			newlmi = unique
			*msg.Base().FOriginMsgID = newlmi
			cio.Confirm("NOTE: the local message ID has been changed to %s for uniqueness.", newlmi)
		}
		if lmi != "" {
			incident.RemoveMessage(lmi)
		}
		lmi = newlmi
	}
	// Save the resulting message.
	if err = incident.SaveMessage(lmi, "", env, msg, false, false); err != nil {
		return fmt.Errorf("saving %s: %s", lmi, err)
	}
	// Display the result.
	cio.ListMessage(listItemForMessage(lmi, "", env))
	if lmi == "config" {
		return nil
	}
	// If the message had any fields that we keep in the configuration,
	// update the configuration.
	if msg.Base().FOriginMsgID != nil && *msg.Base().FOriginMsgID != "" {
		config.C.TxMessageID, saveConfig = *msg.Base().FOriginMsgID, true
	}
	if msg.Base().FOpCall != nil && *msg.Base().FOpCall != "" {
		config.C.OpCall, saveConfig = *msg.Base().FOpCall, true
	}
	if msg.Base().FOpName != nil && *msg.Base().FOpName != "" {
		config.C.OpName, saveConfig = *msg.Base().FOpName, true
	}
	if msg.Base().FTacCall != nil && *msg.Base().FTacCall != "" {
		config.C.TacCall, saveConfig = *msg.Base().FTacCall, true
	}
	if msg.Base().FTacName != nil && *msg.Base().FTacName != "" {
		config.C.TacName, saveConfig = *msg.Base().FTacName, true
	}
	if saveConfig {
		config.SaveConfig()
	}
	return nil
}

func newToAddressField(to *string) (f *message.Field) {
	return message.NewAddressListField(&message.Field{
		Label:    "To",
		Value:    to,
		Presence: message.Required,
		EditHelp: "This is the list of addresses to which the message is sent.  Each address must be a JNOS mailbox name, a JNOS category@distribution bulletin address, a BBS network address, or an email address.  The addresses must be separated by commas.  At least one address is required.",
	})
}

func newSendQueueField(fields []*message.Field, ready *bool) (f *message.Field) {
	return message.NewAggregatorField(&message.Field{
		Label:    "Queue for Sending?",
		Choices:  message.Choices{"Yes", "No"},
		EditHelp: "This indicates whether the message should be sent during the next BBS connection.",
		EditValue: func(f *message.Field) string {
			if *ready {
				return "Yes"
			} else {
				return "No"
			}
		},
		EditApply: func(f *message.Field, s string) {
			*ready = strings.HasPrefix(strings.ToLower(s), "y")
		},
		EditSkip: func(*message.Field) bool {
			for _, f := range fields {
				if p := f.EditValid(f); p != "" {
					*ready = false
					return true
				}
			}
			if *ready {
				return true
			}
			*ready = true
			return false
		},
	})
}
