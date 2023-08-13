package cmd

import (
	"fmt"
	"strings"

	"github.com/rothskeller/packet-cmd/terminal"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:                   "list",
	Aliases:               []string{"l"},
	DisableFlagsInUseLine: true,
	Short:                 "List all messages in current directory",
	Long: `The "list" command lists stored messages.  Messages are listed in chronological
order.
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		var (
			remotes map[string]string
			lmis    []string
			err     error
		)
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
			term.ListMessage(li)
		}
		term.EndMessageList("No messages.")
		return nil
	},
}

func listItemForMessage(lmi, rmi string, env *envelope.Envelope) (li *terminal.ListItem) {
	li = new(terminal.ListItem)
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
	if term.Human() {
		if strings.HasPrefix(li.Subject, lmi+"_") {
			li.Subject = li.Subject[len(lmi)+1:]
		} else if rmi != "" && strings.HasPrefix(li.Subject, rmi+"_") {
			li.Subject = li.Subject[len(rmi)+1:]
		}
	}
	return li
}
