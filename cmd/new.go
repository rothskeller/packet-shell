package cmd

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet-shell/config"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/bulletin"
	"github.com/rothskeller/packet/xscmsg/checkin"
	"github.com/rothskeller/packet/xscmsg/checkout"
	"github.com/spf13/pflag"
)

const (
	newSlug = `Create a new outgoing message`
	newHelp = `
usage: packet new ⇥«new-message-type» [«new-message-id»]
       packet new ⇥--copy «message-id» [«new-message-id»]
       packet new ⇥--reply «message-id» [«new-message-type»] [«new-message-id»]
  -c, --copy      ⇥create a copy of an existing message
  -r, --reply     ⇥create a reply to a received message

The "new" (or "n") command creates a new outgoing message.  In interactive (--no-script) mode, the new message will be opened for editing (see "packet help edit" for details).  In --script mode, the local message ID for the new message will be printed to standard output, and subsequent "set" commands can be used to populate it.

When the --reply (or -r) flag is given, the new message will have the same handling order, subject line, and body as the named source message, and its "To" address will be set to the "From" address of the source message.  The new message will have the same message type as the source message unless a «new-message-type» is given on the command line.  If the message type has a "Reference" field, it will be filled with the source message's origin message ID.  The «message-id» must be the local or remote message ID of a received message.  It can be just the numeric part of the ID if that is unique.

When the --copy (or -c) flag is given, the new message will be an exact copy of the named source message except for being given a new local message ID.  The «message-id» must be the local or remote message ID of an outgoing message (either sent or unsent).  It can be just the numeric part of the ID if that is unique.

When neither the --reply nor --copy flag is given, an empty message of «new-message-type» is created.  «new-message-type» must be an unambiguous abbreviation of one of the supported message types.  Use "packet help types" to get a list of supported message types.  A "v2.3" or similar suffix can be added to it (without a space) to specify a particular version of the message type.

If a «new-message-id» is provided on the command line, the new message is created with that local message ID.  The sequence number in it will be incremented as needed to make it unique.  The «new-message-id» may be just an integer, in which case the message number and prefix in the incident / activation configuration are used (see "packet help config").  If no «new-message-id» is given, one will be automatically assigned based on the incident / activation configuration.
`
)

func cmdNew(args []string) (err error) {
	var (
		replyID string
		copyID  string
		nmtype  string
		nmid    string
		msg     message.Message
		flags   = pflag.NewFlagSet("new", pflag.ContinueOnError)
	)
	flags.StringVarP(&replyID, "reply", "r", "", "create a reply to a received message")
	flags.StringVarP(&copyID, "copy", "c", "", "create a copy of an existing message")
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"new"})
	} else if err != nil {
		cio.Error(err.Error())
		return usage(newHelp)
	}
	if err = gaveMutuallyExclusiveFlags(flags, "copy", "reply"); err != nil {
		cio.Error(err.Error())
		return usage(newHelp)
	}
	args = flags.Args()
	switch {
	case copyID != "" && len(args) == 0:
		// nothing
	case copyID != "" && len(args) == 1:
		nmid = args[0]
	case replyID != "" && len(args) == 0:
		// nothing
	case replyID != "" && len(args) == 1:
		nmtype = args[0] // for now; could change to nmid below
	case replyID != "" && len(args) == 2:
		nmtype, nmid = args[0], args[1]
	case copyID == "" && replyID == "" && len(args) == 1:
		nmtype = args[0]
	case copyID == "" && replyID == "" && len(args) == 2:
		nmtype, nmid = args[0], args[1]
	default:
		return usage(newHelp)
	}
	if nmtype != "" {
		if msg, err = msgForTag(nmtype); err != nil {
			if replyID != "" && len(args) == 1 {
				nmid, nmtype = nmtype, ""
			} else {
				cio.Error(err.Error())
				return usage(newHelp)
			}
		}
	}
	switch {
	case nmid == "":
		if (!cio.InputIsTerm || !cio.OutputIsTerm) && config.C.TxMessageID == "" {
			return errors.New("no message numbering pattern defined in configuration; must provide message ID")
		}
	case incident.MsgIDRE.MatchString(nmid):
		// fine
	default:
		if n, err := strconv.Atoi(nmid); err != nil || n <= 0 {
			cio.Error("%q is not a valid message number", nmid)
			return usage(newHelp)
		}
		if (!cio.InputIsTerm || !cio.OutputIsTerm) && config.C.TxMessageID == "" {
			return errors.New("no message numbering pattern defined in configuration; must provide complete message ID")
		}
	}
	return doNew(copyID, replyID, msg, nmid)
}

func doNew(copyID, replyID string, msg message.Message, nmid string) (err error) {
	var (
		lmi    string
		srclmi string
		env    *envelope.Envelope
		srcmsg message.Message
	)
	if copyID != "" {
		if srclmi, err = expandMessageID(copyID, true); err != nil {
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
		cio.Confirm("Creating a new %s as a copy of %s.", msg.Base().Type.Name, srclmi)
	} else {
		env = new(envelope.Envelope)
		if replyID != "" {
			var srcenv *envelope.Envelope

			if srclmi, err = expandMessageID(replyID, true); err != nil {
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
			if msg == nil {
				if msg, err = msgForTag(srcmsg.Base().Type.Tag); err != nil {
					return err
				}
			}
			if srcmsg.Base().FBody != nil && msg.Base().FBody != nil {
				*msg.Base().FBody = *srcmsg.Base().FBody
			}
			if srcmsg.Base().FHandling != nil && msg.Base().FHandling != nil {
				*msg.Base().FHandling = *srcmsg.Base().FHandling
			}
			if srcmsg.Base().FSubject != nil && msg.Base().FSubject != nil {
				*msg.Base().FSubject = *srcmsg.Base().FSubject
			}
			if srcmsg.Base().FOriginMsgID != nil && msg.Base().FReference != nil {
				*msg.Base().FReference = *srcmsg.Base().FOriginMsgID
			}
			cio.Confirm("Creating a new %s as a reply to %s.", msg.Base().Type.Name, srclmi)
		} else {
			cio.Confirm("Creating a new %s.", msg.Base().Type.Name)
		}
		_, env.Bulletin = msg.(*bulletin.Bulletin)
		if env.To == "" {
			env.To = config.C.DefDest
		}
		if msg.Base().FToICSPosition != nil {
			*msg.Base().FToICSPosition = config.C.DefToPosition
		}
		if msg.Base().FToLocation != nil {
			*msg.Base().FToLocation = config.C.DefToLocation
		}
		if msg.Base().FFromICSPosition != nil {
			*msg.Base().FFromICSPosition = config.C.DefFromPosition
		}
		if msg.Base().FFromLocation != nil {
			*msg.Base().FFromLocation = config.C.DefFromLocation
		}
		if msg.Base().FBody != nil && *msg.Base().FBody == "" {
			*msg.Base().FBody = config.C.DefBody
		}
		if msg.Base().FTacCall != nil {
			*msg.Base().FTacCall = config.C.TacCall
		}
		if msg.Base().FTacName != nil {
			*msg.Base().FTacName = config.C.TacName
		}
		if msg.Base().FOpCall != nil {
			*msg.Base().FOpCall = config.C.OpCall
		}
		if msg.Base().FOpName != nil {
			*msg.Base().FOpName = config.C.OpName
		}
	}
	if incident.MsgIDRE.MatchString(nmid) {
		lmi = incident.UniqueMessageID(nmid)
	} else if nmid != "" {
		if match := incident.MsgIDRE.FindStringSubmatch(config.C.TxMessageID); match != nil {
			lmi = incident.UniqueMessageID(match[1] + "-" + nmid + match[3])
		}
	} else if config.C.TxMessageID != "" {
		lmi = incident.UniqueMessageID(config.C.TxMessageID)
	} else if config.C.RxMessageID != "" {
		lmi = incident.UniqueMessageID(config.C.RxMessageID)
	}
	if omi := msg.Base().FOriginMsgID; omi != nil {
		*omi = lmi
	}
	if cio.InputIsTerm && cio.OutputIsTerm {
		return doEdit("", env, msg, "", false)
	}
	if err = incident.SaveMessage(lmi, "", env, msg, false, false); err != nil {
		return fmt.Errorf("saving %s: %s", lmi, err)
	}
	fmt.Println(lmi)
	return nil
}

var aliases = map[string]string{
	"ci": checkin.Type.Tag,
	"co": checkout.Type.Tag,
}

var versionRE = regexp.MustCompile(`v\d(?:[.0-9]+\d)[a-z]*$`)

// msgForTag returns a created message of the type specified by the tag.
func msgForTag(tag string) (msg message.Message, err error) {
	var version string

	if match := versionRE.FindString(tag); match != "" {
		version, tag = match[1:], tag[:len(tag)-len(match)]
	}
	if alias := aliases[tag]; alias != "" {
		tag = alias
	}
	for rt := range message.RegisteredTypes {
		if len(rt) < len(tag) || !strings.EqualFold(tag, rt[:len(tag)]) {
			continue
		}
		if m := message.Create(rt, version); m == nil || !m.Editable() {
			continue
		} else if msg != nil {
			return nil, fmt.Errorf("message type %q is ambiguous", tag)
		} else {
			msg = m
		}
	}
	if msg == nil && version != "" {
		return nil, fmt.Errorf("no such message type \"%sv%s\"", tag, version)
	} else if msg == nil {
		return nil, fmt.Errorf("no such message type %q", tag)
	}
	return msg, nil
}
