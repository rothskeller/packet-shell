package shell

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rothskeller/packet/incident"
)

func cmdQueue(args []string) bool {
	if len(args) != 1 {
		io.WriteString(os.Stderr, "usage: packet queue <message-id>\n")
		return false
	}
	switch matches := expandMessageID(args[0], false); len(matches) {
	case 0:
		fmt.Fprintf(os.Stderr, "ERROR: no such message %q\n", args[0])
	case 1:
		args[0] = matches[0]
	default:
		fmt.Fprintf(os.Stderr, "ERROR: %q is ambiguous (%s)\n", args[0], strings.Join(matches, ", "))
	}
	env, msg, err := incident.ReadMessage(args[0])
	if err != nil {
		return false
	}
	if env.IsFinal() {
		if env.IsReceived() {
			io.WriteString(os.Stderr, "ERROR: message is a received message\n")
		} else {
			io.WriteString(os.Stderr, "ERROR: message has already been sent\n")
		}
		return false
	}
	if len(env.To) == 0 {
		io.WriteString(os.Stderr, "ERROR: message cannot be queued without a \"To\" address.\n")
		return false
	}
	if !env.ReadyToSend {
		env.ReadyToSend = true
		if err = incident.SaveMessage(args[0], "", env, msg); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			return false
		}
	}
	return true
}

func helpQueue() {
	io.WriteString(os.Stdout, `The "queue" command adds an unsent message to the send queue.
    usage: packet queue <message-id>
`)
}

func cmdDraft(args []string) bool {
	if len(args) != 1 {
		io.WriteString(os.Stderr, "usage: packet queue <message-id>\n")
		return false
	}
	switch matches := expandMessageID(args[0], false); len(matches) {
	case 0:
		fmt.Fprintf(os.Stderr, "ERROR: no such message %q\n", args[0])
	case 1:
		args[0] = matches[0]
	default:
		fmt.Fprintf(os.Stderr, "ERROR: %q is ambiguous (%s)\n", args[0], strings.Join(matches, ", "))
	}
	env, msg, err := incident.ReadMessage(args[0])
	if err != nil {
		return false
	}
	if env.IsFinal() {
		if env.IsReceived() {
			io.WriteString(os.Stderr, "ERROR: message is a received message\n")
		} else {
			io.WriteString(os.Stderr, "ERROR: message has already been sent\n")
		}
		return false
	}
	if env.ReadyToSend {
		env.ReadyToSend = false
		if err = incident.SaveMessage(args[0], "", env, msg); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			return false
		}
	}
	return true
}

func helpDraft() {
	io.WriteString(os.Stdout, `The "draft" command removes an unsent message from the send queue.
    usage: packet draft <message-id>
`)
}

func cmdDelete(args []string) bool {
	if len(args) != 1 || !msgIDRE.MatchString(args[0]) {
		io.WriteString(os.Stderr, "usage: packet delete <message-id>\n")
		return false
	}
	env, _, err := incident.ReadMessage(args[0])
	if err != nil {
		return false
	}
	if env.IsFinal() {
		if env.IsReceived() {
			io.WriteString(os.Stderr, "ERROR: can't delete a received message\n")
		} else {
			io.WriteString(os.Stderr, "ERROR: message has already been sent\n")
		}
		return false
	}
	incident.RemoveMessage(args[0])
	return true
}

func helpDelete() {
	io.WriteString(os.Stdout, `The "delete" command deletes an unsent message.
    usage: packet delete <message-id>
For safety's sake, the <message-id> must be fully written out.
`)
}
