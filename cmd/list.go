package cmd

import (
	"fmt"
	"strings"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"

	"github.com/spf13/pflag"
)

const listSlug = `List all messages in current directory`
const listHelp = `
usage: packet list

The "list" (or "l") command lists stored messages.  Messages are listed in chronological order.  If standard output is a terminal, messages are listed in a table; otherwise, they are listed in CSV format.
`

func cmdList(args []string) (err error) {
	var (
		remotes map[string]string
		lmis    []string
	)
	var flags = pflag.NewFlagSet("list", pflag.ContinueOnError)
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"list"})
	} else if err != nil {
		cio.Error(err.Error())
		return usage(listHelp)
	}
	if len(args) != 0 {
		return usage(listHelp)
	}
	// Read the remote message IDs.
	if remotes, err = incident.RemoteMap(); err != nil {
		return fmt.Errorf("read remote message IDs: %s", err)
	}
	// Now read the list of files again and display those that should be
	// displayed.
	if lmis, err = incident.AllLMIs(); err != nil {
		return fmt.Errorf("read list of messages: %s", err)
	}
	for _, lmi := range lmis {
		env, _, err := incident.ReadMessage(lmi)
		if err != nil {
			continue
		}
		li := listItemForMessage(lmi, remotes[lmi], env)
		if !env.IsReceived() && env.IsFinal() && !incident.HasDeliveryReceipt(lmi) {
			li.Flag = "NO RCPT"
		}
		cio.ListMessage(li)
	}
	cio.EndMessageList("No messages.")
	return nil
}

func listItemForMessage(lmi, rmi string, env *envelope.Envelope) (li *cio.ListItem) {
	li = new(cio.ListItem)
	if env.ReceivedArea != "" {
		li.Handling = "B"
	} else {
		_, _, li.Handling, _, _ = message.DecodeSubject(env.SubjectLine)
	}
	if env.IsReceived() {
		li.Time = env.ReceivedDate
	} else if env.IsFinal() {
		li.Time = env.Date
	} else if env.ReadyToSend {
		li.Flag = "QUEUE"
	} else {
		li.Flag = "DRAFT"
	}
	if env.IsReceived() {
		if rmi != "" {
			li.From = rmi
		} else if env.ReceivedArea != "" {
			var from = strings.ToUpper(env.ReceivedArea)
			li.From = strings.Replace(from, "@ALL", "@", 1) // for brevity
		} else if addrs, err := envelope.ParseAddressList(env.From); err == nil {
			var from, _, _ = strings.Cut(addrs[0].Address, "@")
			li.From = strings.ToUpper(from)
		} else {
			li.From = "??????"
		}
	} else {
		if rmi != "" {
			li.To = rmi
		} else {
			if addrs, err := envelope.ParseAddressList(env.To); err != nil && env.To != "" {
				li.To = env.To
			} else if len(addrs) == 0 {
				li.To = "??????   "
			} else {
				to, _, _ := strings.Cut(addrs[0].Address, "@")
				li.To = strings.ToUpper(to)
			}
		}
	}
	li.LMI = lmi
	li.Subject = env.SubjectLine
	if cio.OutputIsTerm {
		if strings.HasPrefix(li.Subject, lmi+"_") {
			li.Subject = li.Subject[len(lmi)+1:]
		} else if rmi != "" && strings.HasPrefix(li.Subject, rmi+"_") {
			li.Subject = li.Subject[len(rmi)+1:]
		}
	}
	return li
}
