package shell

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/checkin"
	"github.com/rothskeller/packet/xscmsg/checkout"
)

// cmdNew implements the new command.
func cmdNew(args []string) bool {
	var (
		tag string
		msg message.Message
	)
	// Check arguments.  For convenience, aliases are available for the
	// Check-In and Check-Out message types, which would otherwise have to
	// be fully typed out because they are so similar.
	if len(args) == 1 {
		tag = args[0]
		if tag == "ci" {
			tag = checkin.Type.Tag
		}
		if tag == "co" {
			tag = checkout.Type.Tag
		}
	} else {
		io.WriteString(os.Stderr, "usage: packet new <message-type>\n")
		return false
	}
	// Translate the tag (prefix) into a message type.
	if msg = msgForTag(tag); msg == nil {
		return false
	}
	// If the message has a default body field, put the default text in it.
	if mb := msg.Base(); mb.FBody != nil {
		*mb.FBody = config.DefBody
	}
	return newAndReply(new(envelope.Envelope), msg)
}

var aliases = map[string]string{
	checkin.Type.Tag:  "ci",
	checkout.Type.Tag: "co",
}

func helpNew() {
	io.WriteString(os.Stdout, `The "new" command creates a draft outgoing message.
    usage: packet new <message-type>
<message-type> must be an unambiguous abbreviation of one of these types:
`)
	var tags = make([]string, 0, len(message.RegisteredTypes))
	var taglen int
	for tag := range message.RegisteredTypes {
		if msg := message.Create(tag); msg == nil || !msg.Editable() {
			continue
		}
		tags = append(tags, tag)
		aliaslen := len(aliases[tag])
		if aliaslen != 0 {
			aliaslen += 3
		}
		if len(tag)+aliaslen > taglen {
			taglen = len(tag) + aliaslen
		}
	}
	sort.Strings(tags)
	for _, tag := range tags {
		if alias := aliases[tag]; alias != "" {
			fmt.Printf("    %-*s  (%s)\n", taglen, tag+" ("+alias+")", message.RegisteredTypes[tag].Name)
		} else {
			fmt.Printf("    %-*s  (%s)\n", taglen, tag, message.RegisteredTypes[tag].Name)
		}
	}
	io.WriteString(os.Stdout, `The new message is opened in an editor; run "help edit" for details.
`)
}

func cmdReply(args []string) bool {
	var (
		recvid string
		tag    string
		renv   *envelope.Envelope
		rmsg   message.Message
		senv   envelope.Envelope
		smsg   message.Message
		err    error
	)
	// Parse arguments.
	switch len(args) {
	case 1:
		recvid = args[0]
	case 2:
		recvid, tag = args[0], args[1]
	default:
		fmt.Fprintf(os.Stderr, "usage: packet reply <message-id> [<message-type>]\n")
		return false
	}
	// Find the received message we're replying to.
	switch lmis := expandMessageID(recvid, true); len(lmis) {
	case 0:
		fmt.Fprintf(os.Stderr, "ERROR: no such message %q\n", recvid)
		return false
	case 1:
		if renv, rmsg, err = incident.ReadMessage(lmis[0]); err != nil {
			return false
		}
		if !renv.IsReceived() {
			fmt.Fprintf(os.Stderr, "ERROR: message %s is not a received message\n", lmis[0])
			return false
		}
		if renv.ReceivedArea != "" {
			io.WriteString(os.Stderr, "ERROR: cannot reply to a bulletin message\n")
			return false
		}
	default:
		fmt.Fprintf(os.Stderr, "ERROR: %q is ambiguous (%s)\n", recvid, strings.Join(lmis, ", "))
		return false
	}
	// Set up the envelope for the new message.
	senv.To = []string{renv.From}
	// Create the empty message of the appropriate type.
	if tag != "" {
		if smsg = msgForTag(tag); smsg == nil {
			return false
		}
	} else if rmsg.Editable() {
		smsg = message.Create(rmsg.Base().Type.Tag)
	} else {
		fmt.Fprintf(os.Stderr, "ERROR: can't create %s %s; specify some other message type\n",
			rmsg.Base().Type.Article, rmsg.Base().Type.Name)
		return false
	}
	// If the message has a default body field, put the default text in it.
	// (We may overwrite this below if the received message has a body.)
	if sb := smsg.Base(); sb.FBody != nil {
		*sb.FBody = config.DefBody
	}
	// Copy over the data from the received message to the reply.
	if rb, sb := rmsg.Base(), smsg.Base(); rb.FHandling != nil && sb.FHandling != nil {
		*sb.FHandling = *rb.FHandling
	}
	if rb, sb := rmsg.Base(), smsg.Base(); rb.FSubject != nil && sb.FSubject != nil {
		*sb.FSubject = *rb.FSubject
	}
	if rb, sb := rmsg.Base(), smsg.Base(); rb.FOriginMsgID != nil && sb.FReference != nil {
		*sb.FReference = *rb.FOriginMsgID
	}
	if rb, sb := rmsg.Base(), smsg.Base(); rb.FBody != nil && *rb.FBody != "" && sb.FBody != nil {
		*sb.FBody = *rb.FBody
	}
	return newAndReply(&senv, smsg)
}

func helpReply() {
	io.WriteString(os.Stdout, `The "reply" command starts a reply message.
    usage: packet reply <message-id> [<message-type>]
The "reply" command creates a new draft message with the same handling order
and subject as the received message identified by <message-id>.  The reply's
"To" address is set to the received message's "From" address.
    The reply is the same type of message as the received message, unless a
different <message-type> is specified.  (Enter "help new" for a list of
message types.)
    If the received message is a body-centric type (e.g., plain text or
ICS-213), its body is copied into the reply message.
    If the reply message type has a "Reference" field, it is set to the origin
message ID of the received message.
    The new message is opened in an editor; run "help edit" for details.
`)
}

func cmdResend(args []string) bool {
	var (
		env *envelope.Envelope
		msg message.Message
		err error
	)
	// Parse arguments.
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: packet resend <message-id>\n")
		return false
	}
	// Find the outgoing message we're resending.
	switch lmis := expandMessageID(args[0], true); len(lmis) {
	case 0:
		fmt.Fprintf(os.Stderr, "ERROR: no such message %q\n", args[0])
		return false
	case 1:
		if env, msg, err = incident.ReadMessage(lmis[0]); err != nil {
			return false
		}
		if env.IsReceived() {
			fmt.Fprintf(os.Stderr, "ERROR: message %s is not a outgoing message\n", lmis[0])
			return false
		}
		if !msg.Editable() {
			fmt.Fprintf(os.Stderr, "ERROR: %ss are not editable\n", msg.Base().Type.Name)
			return false
		}
	default:
		fmt.Fprintf(os.Stderr, "ERROR: %q is ambiguous (%s)\n", args[0], strings.Join(lmis, ", "))
		return false
	}
	// Give the message a new number, and mark it as a draft.
	env.ReadyToSend = false
	if mb := msg.Base(); mb.FOriginMsgID != nil {
		*mb.FOriginMsgID = ""
	}
	return newAndReply(env, msg)
}

func helpResend() {
	io.WriteString(os.Stdout, `The "resend" command creates a new, draft message as a copy of a sent message.
    usage: packet resend <message-id>
The "resend" command creates a new draft message with the same content as the
existing outgoing message identified by <message-id>.
    The new message is opened in an editor; run "help edit" for details.
`)
}

// msgForTag returns a created message of the type specified by the tag.  It
// returns nil if the tag is invalid.
func msgForTag(tag string) (msg message.Message) {
	for rt := range message.RegisteredTypes {
		if len(rt) < len(tag) || !strings.EqualFold(tag, rt[:len(tag)]) {
			continue
		}
		if m := message.Create(rt); m == nil || !m.Editable() {
			continue
		} else if msg != nil {
			fmt.Fprintf(os.Stderr, "ERROR: message type %q is ambiguous\n", tag)
			return nil
		} else {
			msg = m
		}
	}
	if msg == nil {
		fmt.Fprintf(os.Stderr, "ERROR: no such message type %q\n", tag)
	}
	return msg
}

// newAndReply is the common code shared by new and reply.
func newAndReply(env *envelope.Envelope, msg message.Message) bool {
	// If we have a message ID pattern, give the new message an ID.
	// Otherwise we'll leave it blank and the editor will force the user to
	// provide one.
	if config.MessageID != "" {
		*msg.Base().FOriginMsgID = incident.UniqueMessageID(config.MessageID)
	}
	// Special case for check-in and check-out messages: fill in the call
	// signs and names from the incident/activation settings.  If we don't
	// have any yet, no harm done.
	switch msg := msg.(type) {
	case *checkin.CheckIn:
		msg.TacticalCallSign, msg.TacticalStationName = config.TacCall, config.TacName
		msg.OperatorCallSign, msg.OperatorName = config.OpCall, config.OpName
	case *checkout.CheckOut:
		msg.TacticalCallSign, msg.TacticalStationName = config.TacCall, config.TacName
		msg.OperatorCallSign, msg.OperatorName = config.OpCall, config.OpName
	}
	// Run the editor on the message.  Note that the editor is told there
	// is no LMI, even if we put one in the Origin Message ID field.  That
	// gives the editor the correct title bar, and disables the LMI change
	// behavior in case the user edits the Origin Message ID field.
	if !editMessage("", env, msg, "", false) {
		return false
	}
	// If we don't have a message ID model in the incident/activation
	// settings yet, use the Origin Message ID of the new message as our
	// model going forward.
	if config.MessageID == "" {
		setSetting([]string{"msgid", *msg.Base().FOriginMsgID})
	}
	// If this was a check-in message, and we don't already have operator
	// and tactical information in the incident/activation settings, store
	// the values from the message into the settings.
	if msg, ok := msg.(*checkin.CheckIn); ok {
		if config.OpCall == "" && fccCallSignRE.MatchString(msg.OperatorCallSign) && msg.OperatorName != "" {
			setSetting([]string{"operator", msg.OperatorCallSign, msg.OperatorName})
		}
		if !config.TacRequested && config.TacCall == "" {
			if tacCallSignRE.MatchString(msg.TacticalCallSign) && msg.TacticalStationName != "" {
				setSetting([]string{"tactical", msg.TacticalCallSign, msg.TacticalStationName})
			} else if msg.TacticalCallSign == "" {
				setSetting([]string{"tactical"}) // so we don't ask for it
			}
		}
	}
	return true
}
