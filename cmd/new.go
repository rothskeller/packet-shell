package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rothskeller/packet-cmd/config"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/checkin"
	"github.com/rothskeller/packet/xscmsg/checkout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newCmd)
	newCmd.Flags().StringP("reply", "r", "", "create a reply to a received message")
	newCmd.Flags().StringP("copy", "c", "", "create a copy of an existing message")
	newCmd.MarkFlagsMutuallyExclusive("reply", "copy")
}

var aliases = map[string]string{
	checkin.Type.Tag:  "ci",
	checkout.Type.Tag: "co",
}

var newCmd = &cobra.Command{
	Use:                   "new [--reply «message-id»] [--copy «message-id»] [«message-type»]",
	Aliases:               []string{"n"},
	SuggestFor:            []string{"reply", "copy", "resend"},
	Short:                 "Create a new outgoing message",
	Args:                  cobra.MaximumNArgs(1),
	DisableFlagsInUseLine: true,
	Long: `
The "new" command creates a new outgoing message.  In interactive mode, the
new message will be opened for editing (see "packet help edit" for details).
In scripted mode, the local message ID for the new message will be printed to
standard output, and subsequent "set" commands can be used to populate it.

When the --reply (or -r) flag is given, the new message will have the same
handling order, subject line, and body as the named source message, and its
"To" address will be set to the "From" address of the source message.  The
new message will have the same message type as the source message unless a
different «message-type» is given on the command line.  If the message type
has a "Reference" field, it will be filled with the source message's origin
message ID.  The «message-id» must be the local or remote message ID of a
received message.  It can be just the numeric part of the ID if that is
unique.

When the --copy (or -c) flag is given, the new message will be an exact copy
of the named source message except for being given a new local message ID.
No «message-type» can be given when --copy is given.  The «message-id» must be
the local or remote message ID of an outgoing message (either sent or unsent).
It can be just the numeric part of the ID if that is unique.

When neither the --reply nor --copy flag is given, the «message-type» is
required, and the new message is created with that type.  <message-type> must
be an unambiguous abbreviation of one of the supported message types.  Use
"packet help types" to get a list of supported message types.
`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			srclmi  string
			typetag string
			env     *envelope.Envelope
			msg     message.Message
		)
		if len(args) > 0 {
			typetag = args[0]
		}
		if srclmi, err = cmd.Flags().GetString("copy"); err == nil && srclmi != "" {
			if typetag != "" {
				return errors.New("cannot specify message type when using --copy")
			}
			if srclmi, err = expandMessageID(srclmi, true); err != nil {
				return err
			}
			if env, msg, err = incident.ReadMessage(srclmi); err != nil {
				return fmt.Errorf("reading %s: %s", srclmi, err)
			}
			if !msg.Editable() {
				return fmt.Errorf("%ss do not support editing", msg.Base().Type.Tag)
			}
			if env.IsReceived() {
				env = &envelope.Envelope{SubjectLine: env.SubjectLine}
			} else {
				env = &envelope.Envelope{To: env.To, SubjectLine: env.SubjectLine}
			}
			if config.C.MessageID != "" {
				*msg.Base().FOriginMsgID = incident.UniqueMessageID(config.C.MessageID)
			} else {
				*msg.Base().FOriginMsgID = ""
			}
		} else {
			var srcmsg message.Message

			env = new(envelope.Envelope)
			if srclmi, err = cmd.Flags().GetString("reply"); err == nil && srclmi != "" {
				var srcenv *envelope.Envelope

				if srclmi, err = expandMessageID(srclmi, true); err != nil {
					return err
				}
				if srcenv, srcmsg, err = incident.ReadMessage(srclmi); err != nil {
					return fmt.Errorf("reading %s: %s", srclmi, err)
				}
				if !srcenv.IsReceived() {
					return fmt.Errorf("%s is not a received message", srclmi)
				}
				env.To = srcenv.From
				env.SubjectLine = srcenv.SubjectLine
				if typetag == "" {
					typetag = srcmsg.Base().Type.Tag
				}
			} else if typetag == "" {
				return errors.New("must specify message type unless using --copy or --reply")
			}
			if msg, err = msgForTag(typetag); err != nil {
				return err
			}
			if srcmsg != nil && srcmsg.Base().FBody != nil && msg.Base().FBody != nil {
				*msg.Base().FBody = *srcmsg.Base().FBody
			}
			if srcmsg != nil && srcmsg.Base().FHandling != nil && msg.Base().FHandling != nil {
				*msg.Base().FHandling = *srcmsg.Base().FHandling
			}
			if srcmsg != nil && srcmsg.Base().FSubject != nil && msg.Base().FSubject != nil {
				*msg.Base().FSubject = *srcmsg.Base().FSubject
			}
			if srcmsg != nil && srcmsg.Base().FOriginMsgID != nil && msg.Base().FReference != nil {
				*msg.Base().FReference = *srcmsg.Base().FOriginMsgID
			}
			if env.To == "" {
				env.To = config.C.DefDest
			}
			if msg.Base().FBody != nil && *msg.Base().FBody == "" {
				*msg.Base().FBody = config.C.DefBody
			}
			if config.C.MessageID != "" {
				*msg.Base().FOriginMsgID = incident.UniqueMessageID(config.C.MessageID)
			}
		}
		if term.Human() {
			return doEdit("", env, msg, "", false)
		}
		if *msg.Base().FOriginMsgID == "" {
			return errors.New("must set message ID pattern with \"packet config msgid\" first")
		}
		if err = incident.SaveMessage(*msg.Base().FOriginMsgID, "", env, msg); err != nil {
			return fmt.Errorf("saving %s: %s", *msg.Base().FOriginMsgID, err)
		}
		fmt.Println(*msg.Base().FOriginMsgID)
		return nil
	},
}

// msgForTag returns a created message of the type specified by the tag.
func msgForTag(tag string) (msg message.Message, err error) {
	for rt := range message.RegisteredTypes {
		if len(rt) < len(tag) || !strings.EqualFold(tag, rt[:len(tag)]) {
			continue
		}
		if m := message.Create(rt); m == nil || !m.Editable() {
			continue
		} else if msg != nil {
			return nil, fmt.Errorf("message type %q is ambiguous", tag)
		} else {
			msg = m
		}
	}
	if msg == nil {
		return nil, fmt.Errorf("no such message type %q", tag)
	}
	return msg, nil
}
