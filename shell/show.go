package shell

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/rothskeller/packet-cmd/editfield"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
)

// cmdShow implements the "show" command with the specified arguments.  The
// format argument is usually blank, but can be used force the format.
func cmdShow(args []string, format string) bool {
	var msgid string

	if format == "" && len(args) == 2 {
		msgid, format = args[0], args[1]
	} else if len(args) == 1 {
		msgid = args[0]
	} else {
		helpShow()
		return false
	}
	switch lmis := expandMessageID(msgid, true); len(lmis) {
	case 0:
		io.WriteString(os.Stderr, "ERROR: no such message\n")
		return false
	case 1:
		env, msg, err := incident.ReadMessage(lmis[0])
		if err != nil {
			return false
		}
		return showMessage(lmis[0], env, msg, format)
	default:
		fmt.Fprintf(os.Stderr, "ERROR: %q is ambiguous (%s)\n", msgid, strings.Join(lmis, ", "))
		return false
	}
}

func helpShow() {
	io.WriteString(os.Stdout, `The "show" command displays a message.
    usage: packet show <message-id> [<format>]
           packet pdf <message-id>
<message-id> is the local or remote message ID of the message to show.
    It can be just the numeric part if that is unambiguous.
<format> is one of:
    "table" or "t" (the default): flat text table of field names and values
    "raw" or "r":  the PackItForms- and RFC-5322-encoded message
    "pdf" or "p":  PDF rendering of the form (opens in system PDF viewer)
If the message to be shown is a received or sent message (i.e., not a draft or
queued message), the "show" command word can be omitted entirely.
    The "pdf" command (which cannot be abbreviated) is equivalent to the
"show" command with a "pdf" <format>.
`)
}

// showMessage displays a message in the requested format.  It returns true if
// successful.
func showMessage(lmi string, env *envelope.Envelope, msg message.Message, format string) bool {
	switch format {
	case "", "table", "t":
		return showAsTable(lmi, env, msg.Base())
	case "raw", "r":
		showAsRaw(env, msg)
		return true
	case "pdf", "p":
		return showAsPDF(lmi)
	default:
		fmt.Fprintf(os.Stderr, "ERROR: no such format %q\n", format)
		return false
	}
}

// showAsRaw displays a message in PackItForms- and RFC-5322-encoded storage
// format.
func showAsRaw(env *envelope.Envelope, msg message.Message) {
	io.WriteString(os.Stdout, env.RenderSaved(msg.EncodeBody()))
}

// showAsTable displays a message in a plain text tabular format.  It returns
// true if successful.
func showAsTable(lmi string, env *envelope.Envelope, msg *message.BaseMessage) bool {
	var labels, values []string

	labels = append(labels, "Message Type")
	values = append(values, strings.ToUpper(msg.Type.Name[:1])+msg.Type.Name[1:])
	if env.IsReceived() {
		labels = append(labels, "From", "Sent", "To", "Received")
		values = append(values, env.From, env.Date.Format("01/02/2006 15:04"), strings.Join(env.To, ", "),
			fmt.Sprintf("%s as %s", env.ReceivedDate.Format("01/02/2006 15:04"), lmi))
	} else {
		if len(env.To) != 0 {
			labels, values = append(labels, "To"), append(values, strings.Join(env.To, ", "))
		}
		if !env.Date.IsZero() {
			labels, values = append(labels, "Sent"), append(values, env.Date.Format("01/02/2006 15:04"))
		}
	}
	for _, f := range msg.Fields {
		if value := f.TableValue(f); f.Label != "" && value != "" {
			labels, values = append(labels, f.Label), append(values, value)
		}
	}
	var labellen int
	for _, l := range labels {
		if len(l) > labellen {
			labellen = len(l)
		}
	}
	e := editfield.NewEditor(labellen)
	for i, l := range labels {
		e.Display(l, values[i])
	}
	return true
}

// showAsPDF opens the system PDF viewer to view the PDF rendering of a message.
// It returns true if successful.
func showAsPDF(lmi string) bool {
	var txtFI, pdfFI os.FileInfo
	var err error

	// Check to be sure that the PDF is newer than the TXT.  If not, it
	// needs to be regenerated.
	if txtFI, err = os.Stat(lmi + ".txt"); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		return false
	}
	if pdfFI, err = os.Stat(lmi + ".pdf"); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		return false
	}
	if pdfFI == nil || pdfFI.ModTime().Before(txtFI.ModTime()) {
		// PDF needs to be regenerated.
		var msg message.Message
		if _, msg, err = incident.ReadMessage(lmi); err != nil {
			return false
		}
		if err = msg.RenderPDF(lmi + ".pdf"); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			return false
		}
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd.exe", "/C", lmi+".pdf")
	case "darwin":
		cmd = exec.Command("open", lmi+".pdf")
	default:
		cmd = exec.Command("xdg-open", lmi+".pdf")
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: PDF viewer could not be started: %s\n", err)
		return false
	}
	go func() { cmd.Wait() }()
	return true
}
