package shell

import (
	"fmt"
	"io"
	"net/mail"
	"os"
	"strings"

	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message/common"
	"golang.org/x/term"
)

// cmdList implements the list command.
func cmdList(args []string) bool {
	var (
		remotes map[string]string
		lmis    []string
		l       lister
		err     error
	)
	if len(args) != 0 {
		io.WriteString(os.Stderr, "usage: packet list\n")
		return false
	}
	// Read the remote message IDs.
	if remotes, err = incident.RemoteMap(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		return false
	}
	// Now read the list of files again and display those that should be
	// displayed.
	if lmis, err = incident.AllLMIs(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		return false
	}
	for _, lmi := range lmis {
		env, _, err := incident.ReadMessage(lmi)
		if err != nil {
			continue
		}
		l.item(lmi, remotes[lmi], env, incident.HasDeliveryReceipt(lmi))
	}
	if !l.seenOne {
		io.WriteString(os.Stdout, "No messages.\n")
	}
	return true
}

func helpList() {
	io.WriteString(os.Stdout, `The "list" command lists stored messages.
    usage: packet list
Messages are listed in chronological order.
When the packet command is running as a shell, entering a blank line at the
"packet>" prompt invokes the list command.
`)
}

type lister struct {
	width   int
	seenOne bool
}

func (l *lister) item(lmi, rmi string, env *envelope.Envelope, hasDR bool) {
	var color string

	if l.width == 0 {
		l.width, _, _ = term.GetSize(int(os.Stdout.Fd()))
	}
	if l.width == 0 {
		l.width = 80
	}
	if !l.seenOne {
		io.WriteString(os.Stdout, "\033[1mTIME  FROM        LOCAL ID    TO         SUBJECT\033[0m\n") // bold
		l.seenOne = true
	}
	// Set the background color for the item based on its handling.
	if env.ReceivedArea != "" {
		color = "\033[38;5;14m" // cyan
	} else {
		_, _, handling, _, _ := common.DecodeSubject(env.SubjectLine)
		if handling == "I" {
			color = "\033[38;5;202m" // red
		} else if handling == "P" {
			color = "\033[38;5;11m" // yellow
		}
	}
	// Write the time column.
	if env.IsReceived() {
		io.WriteString(os.Stdout, color)
		io.WriteString(os.Stdout, env.ReceivedDate.Format("15:04 "))
	} else if env.IsFinal() {
		io.WriteString(os.Stdout, color)
		io.WriteString(os.Stdout, env.Date.Format("15:04 "))
	} else if env.ReadyToSend {
		io.WriteString(os.Stdout, "\033[38;5;0;48;5;11mQUEUE\033[0m ") // black on yellow
		io.WriteString(os.Stdout, color)
	} else {
		io.WriteString(os.Stdout, "\033[38;5;15;48;5;9mDRAFT\033[0m ") // white on red
		io.WriteString(os.Stdout, color)
	}
	// Write the from column.
	if env.IsReceived() {
		var from string
		if rmi != "" {
			from = rmi
		} else if env.ReceivedArea != "" {
			from = strings.ToUpper(env.ReceivedArea)
			from = strings.Replace(from, "@ALL", "@", 1) // for brevity
		} else if addr, err := mail.ParseAddress(env.From); err == nil {
			from, _, _ = strings.Cut(addr.Address, "@")
			from = strings.ToUpper(from)
		} else {
			from = "??????"
		}
		fmt.Printf("%-9.9s → ", from)
	} else if env.IsFinal() && !hasDR {
		io.WriteString(os.Stdout, "\033[38;5;0;48;5;11mNO RCPT\033[0m     ") // black on yellow
		io.WriteString(os.Stdout, color)
	} else {
		io.WriteString(os.Stdout, "            ")
	}
	// Write the LMI column.
	fmt.Printf("%-9.9s", lmi)
	// Write the to column.
	if env.IsReceived() {
		io.WriteString(os.Stdout, "              ")
	} else {
		var to = "??????"
		if rmi != "" {
			to = rmi
		} else if len(env.To) != 0 {
			if addr, err := mail.ParseAddress(env.To[0]); err == nil {
				to = addr.Address
			} else {
				to = env.To[0]
			}
			to, _, _ = strings.Cut(to, "@")
			to = strings.ToUpper(to)
		}
		fmt.Printf(" → %-9.9s  ", to)
	}
	// Write the subject column.
	subject := env.SubjectLine
	if strings.HasPrefix(subject, lmi+"_") {
		subject = subject[len(lmi)+1:]
	} else if rmi != "" && strings.HasPrefix(subject, rmi+"_") {
		subject = subject[len(rmi)+1:]
	}
	if len(subject) > l.width-42 {
		subject = subject[:l.width-42]
	}
	io.WriteString(os.Stdout, subject)
	io.WriteString(os.Stdout, "\033[0m\n")
}
