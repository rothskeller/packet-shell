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
)

const (
	setSlug = `Set the value of a field of a message`
	setHelp = `
usage: packet set ⇥[flags] «message-id»|config «field-name» [«value»]
  --force  ⇥allow invalid value

The "set" command sets the value of a field of a message.  If a «value» is provided on the command line, it is used; otherwise, the new value is read from standard input.  The provided value must be valid for the field unless the --force flag is given.

«message-id» must be the local message ID of an unsent outgoing message.  It can be just the numeric part of the ID if that is unique.  If the word "config" (or an abbreviation) is used instead, the "set" command sets variables in the incident / activation settings (see "packet help config").

«field-name» is the name of the field to set.  It can be the PackItForms tag for the field (including the trailing period, if any), or it can be the full field name.  In interactive (--no-script) mode, it can be a shortened version of the field name, such as "ocs" for "Operator Call Sign."
`
)

func cmdSet(args []string) (err error) {
	var (
		force     bool
		lmi       string
		env       *envelope.Envelope
		msg       message.Message
		fields    []*message.Field
		field     *message.Field
		problems  map[*message.Field]string
		newprob   bool
		lmichange string
		fastsave  bool
		flags     = pflag.NewFlagSet("set", pflag.ContinueOnError)
	)
	flags.BoolVar(&force, "force", false, "allow invalid value")
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"set"})
	} else if err != nil {
		cio.Error("%s", err.Error())
		return usage(setHelp)
	}
	args = flags.Args()
	if len(args) < 2 {
		return usage(setHelp)
	}
	if strings.HasPrefix("config", args[0]) {
		lmi, msg = "config", &config.C
	} else {
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
	}
	// Build the list of fields that can be set.  This includes the
	// To address list, an all editable fields.  We disregard
	// EditSkip; that allows addressing fields with PIFOTags that
	// are normally aggregated into other fields and not directly
	// editable.
	if lmi != "config" {
		fields = append(fields, newToAddressField(&env.To))
	}
	for _, f := range msg.Base().Fields {
		if f.EditHelp != "" {
			fields = append(fields, f)
		}
	}
	// Verify that we have a valid field to edit.
	if field, err = expandFieldName(fields, args[1], cio.OutputIsTerm && cio.InputIsTerm); err != nil {
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
		value := strings.Join(args[2:], " ")
		value = strings.Map(func(r rune) rune { // ensure pure ASCII
			if (r >= ' ' && r <= '~') || r == '\n' {
				return r
			}
			if r == '\t' {
				return ' '
			}
			return -1
		}, value)
		field.EditApply(field, value)
		if cio.OutputIsTerm {
			cio.ShowNameValue(field.Label, field.EditValue(field), 0)
		} else {
			fastsave = true
		}
	} else if _, err = cio.EditField(field, 0); err != nil {
		return err
	}
	// If we edited the LMI, check it.  We have to have a valid one
	// to save the file.  If they changed it, make sure the new one
	// isn't already in use.
	if lmi != "config" && field.Value == msg.Base().FOriginMsgID {
		if p := field.EditValid(field); p != "" {
			return errors.New(p)
		}
		newlmi := *field.Value
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
			cio.Error("%s", p)
			newprob = true
		}
	}
	if newprob {
		if force {
			cio.Confirm("NOTE: applying the changes anyway since --force was used")
			if lmi != "config" && env.ReadyToSend && fields[0].EditValid(fields[0]) != "" {
				env.ReadyToSend = false
				cio.Confirm("NOTE: removing from send queue; can't send without valid To address")
			}
		} else {
			return errors.New("change not applied; use --force to override")
		}
	}
	// Apply the change.
	if lmi == "config" {
		config.SaveConfig()
		incident.RemoveICS309s()
		return nil
	}
	if lmichange != "" {
		if err = incident.SaveMessage(lmichange, "", env, msg, fastsave, false); err != nil {
			return fmt.Errorf("saving %s: %s", lmichange, err)
		}
		incident.RemoveMessage(lmi)
		return nil
	}
	if err = incident.SaveMessage(lmi, "", env, msg, fastsave, false); err != nil {
		return fmt.Errorf("saving %s: %s", lmi, err)
	}
	return nil
}
