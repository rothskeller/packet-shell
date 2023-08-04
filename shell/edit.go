package shell

import (
	"fmt"
	"io"
	"net/mail"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/rothskeller/packet-cmd/editfield"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"golang.org/x/exp/constraints"
	"golang.org/x/term"
)

// cmdEdit implements the edit command.
func cmdEdit(args []string) bool {
	var (
		lmi        string
		startAt    string
		errorsOnly bool
		env        *envelope.Envelope
		msg        message.Message
		emsg       message.IEdit
		err        error
	)
	// Check arguments
	switch len(args) {
	case 1:
		lmi = args[0]
	case 2:
		lmi, startAt = args[0], args[1]
	default:
		io.WriteString(os.Stderr, "usage: packet edit <message-id>\n")
		return false
	}
	if startAt == "errors" {
		startAt, errorsOnly = "", true
	}
	switch lmis := expandMessageID(lmi, false); len(lmis) {
	case 0:
		fmt.Fprintf(os.Stderr, "ERROR: no such message %q\n", args[0])
		return false
	case 1:
		lmi = lmis[0]
	default:
		fmt.Fprintf(os.Stderr, "ERROR: %q is ambiguous (%s)\n", args[0], strings.Join(lmis, ", "))
		return false
	}
	// Read and parse file to be edited.
	if env, msg, err = incident.ReadMessage(lmi); err != nil {
		return false
	}
	if m, ok := msg.(message.IEdit); ok {
		emsg = m
	} else {
		fmt.Fprintf(os.Stderr, "ERROR: editing is not supported for %ss\n", msg.Type().Name)
		return false
	}
	if env.IsFinal() {
		if env.IsReceived() {
			io.WriteString(os.Stderr, "ERROR: cannot edit a received message\n")
		} else {
			io.WriteString(os.Stderr, "ERROR: cannot edit a message that has been sent\n")
		}
		return false
	}
	// Start the editor.
	return editMessage(lmi, env, emsg, startAt, errorsOnly)
}

// helpEdit prints the help message for the edit command.
func helpEdit() {
	io.WriteString(os.Stdout, `The "edit" (or "e") command edits an unsent message.
    usage: packet edit <message-id> [<field>|errors]
<message-id> is the local message ID of the message to edit.  It can be just
the numeric part if that is unambiguous.
    <field> is the name (or partial name) of the field to edit first.  If it is
not specified or cannot be recognized, editing starts with the first field.
    The "errors" keyword restricts editing to only those fields with validation
errors.  Without it, all fields are edited.
    Editing is the default action for unsent messages, so the "edit" keyword can
be omitted.
    In the message editor, use Tab and Shift-Tab to move backward and forward in
the list of fields.  (Enter usually moves forward also, except in multi-line
text fields, where it introduces a newline.)  Editing ends when you finish the
last field or press the ESC key.  Press F1 for help on the current field.
    If the result of the edit has no validation errors, and the message is not
already in the send queue, the message editor asks whether to queue it.
Messages with validation errors are removed from the send queue.
`)
}

// editMessage starts a message editor for a message.
func editMessage(lmi string, env *envelope.Envelope, msg message.IEdit, startAt string, errorsOnly bool) bool {
	var (
		fields     []*message.EditField
		labelWidth int
		state      *term.State
		editor     *editfield.Editor
		fieldidx   int
		err        error
		field      *message.EditField
		nvalue     string
		changed    bool
		sawerror   bool
		result     editfield.Result
	)
	// Get a list of edit fields.
	fields = msg.EditFields()
	fields = append(fields, nil)
	copy(fields[1:], fields)
	fields[0] = &message.EditField{
		Label: "To",
		Value: strings.Join(env.To, ", "),
		Width: 80,
		Help:  "This is the list of addresses to which the message is sent.  Each address must be a JNOS mailbox name, a BBS network address, or an email address.  The addresses must be separated by commas.  At least one address is required.",
	}
	applyToField(fields[0], env) // set initial Problem
	// Find the longest field label.
	for _, f := range fields {
		labelWidth = max(labelWidth, len(f.Label))
	}
	// Create an editor.
	state, _ = term.MakeRaw(int(os.Stdin.Fd()))
	defer term.Restore(int(os.Stdin.Fd()), state)
	editor = editfield.NewEditor(labelWidth)
	// Edit the fields in order.
	fieldidx, field = startingField(fields, startAt)
	for {
		nvalue, changed, result = editor.Edit(
			field.Label, field.Value, field.Help, field.Hint, field.Width, field.Choices, field.Multiline)
		if result == editfield.ResultError || result == editfield.ResultCtrlC {
			editor.Display(field.Label, field.Value)
			break
		}
		field.Value = nvalue
		if fieldidx == 0 {
			applyToField(field, env)
		} else {
			msg.ApplyEdits()
		}
		editor.Display(field.Label, field.Value)
		if field.Problem != "" && (!sawerror || changed) {
			editor.DisplayError(field.Problem)
			sawerror = true
			continue
		}
		sawerror = false
		if result == editfield.ResultEnter || result == editfield.ResultTab {
			fieldidx++
			if errorsOnly {
				for fieldidx < len(fields) && fields[fieldidx].Problem == "" {
					fieldidx++
				}
			}
			if fieldidx >= len(fields) {
				break
			}
		} else if result == editfield.ResultBackTab {
			fieldidx--
			if errorsOnly {
				for fieldidx >= 0 && fields[fieldidx].Problem == "" {
					fieldidx--
				}
			}
			if fieldidx < 0 {
				break
			}
		} else {
			break
		}
		field = fields[fieldidx]
	}
	// Make sure we have a valid LMI.  We have to have one to save the file.
	newlmi := msg.GetOriginID()
	if !msgIDRE.MatchString(newlmi) {
		if lmi != "" {
			newlmi = lmi // restore the one it had when we started
		} else {
			newlmi = incident.UniqueMessageID("AAA-001")
		}
		msg.SetOriginID(newlmi)
	}
	// Does the message have any errors?
	var canqueue = true
	for _, f := range fields {
		if f.Problem != "" {
			canqueue = false
			break
		}
	}
	if canqueue != env.ReadyToSend {
		if canqueue && result != editfield.ResultError && result != editfield.ResultCtrlC {
			if editor.YesNo("Queue for sending?", true) {
				env.ReadyToSend = true
			}
		} else {
			env.ReadyToSend = false
			editor.DisplayError(`This message has invalid fields.  It has been removed from the send queue.  To send it with errors, use the "queue" command to re-queue it.`)
		}
	}
	// Check for a change to the LMI.
	if newlmi != lmi {
		if unique := incident.UniqueMessageID(newlmi); unique != newlmi {
			newlmi = unique
			msg.SetOriginID(newlmi)
		}
		if lmi != "" {
			incident.RemoveMessage(lmi)
		}
		lmi = newlmi
	}
	// Save the resulting message.
	if err = incident.SaveMessage(lmi, "", env, msg); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		return false
	}
	return true
}

func startingField(fields []*message.EditField, startAt string) (idx int, field *message.EditField) {
	var matches []int

	if startAt == "" { // common case
		return 0, fields[0]
	}
	// Look first for matches where all of the startAt characters correspond
	// to word start letters in the field name.
	for i, f := range fields {
		if wordStartMatch(f.Label, startAt) {
			matches = append(matches, i)
		}
	}
	if len(matches) != 0 {
		sort.Slice(matches, func(i, j int) bool {
			return countWords(fields[matches[i]].Label) < countWords(fields[matches[j]].Label)
		})
		return matches[0], fields[matches[0]]
	}
	// Failing that, look for matches where the startAt characters
	// correspond to any character in the field name.
	for i, f := range fields {
		if anyMatch(f.Label, startAt) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return 0, fields[0]
	}
	sort.Slice(matches, func(i, j int) bool {
		return len(fields[matches[i]].Label) < len(fields[matches[j]].Label)
	})
	return matches[0], fields[matches[0]]
}
func wordStartMatch(have, want string) bool {
	return anyMatch(wordStartChars(have), want)
}
func countWords(s string) int {
	return len(strings.Fields(s))
}
func wordStartChars(s string) (chars string) {
	for _, f := range strings.Fields(s) {
		if len(f) >= 2 && f[0] == '(' {
			f = f[1:]
		}
		if (f[0] >= 'A' && f[0] <= 'Z') || (f[0] >= 'a' && f[0] <= 'z') {
			chars += f[:1]
		}
	}
	return chars
}
func anyMatch(have, want string) bool {
	have = strings.ToLower(have)
	want = strings.ToLower(want)
	for want != "" && have != "" {
		if have[0] == want[0] {
			have, want = have[1:], want[1:]
		} else {
			have = have[1:]
		}
	}
	return want == ""
}

var jnosMailboxRE = regexp.MustCompile(`(?i)^[A-Z][A-Z0-9]{0,5}$`)

func applyToField(f *message.EditField, env *envelope.Envelope) {
	addresses := strings.Split(f.Value, ",")
	j := 0
	f.Problem = ""
	for _, address := range addresses {
		if trim := strings.TrimSpace(address); trim != "" {
			addresses[j], j = trim, j+1
			if jnosMailboxRE.MatchString(address) {
				// do nothing
			} else if _, err := mail.ParseAddress(address); err == nil {
				// do nothing
			} else {
				f.Problem = fmt.Sprintf(`The "To" field contains %q, which is not a valid JNOS mailbox name, BBS network address, or email address.`, address)
			}
		}
	}
	f.Value = strings.Join(addresses[:j], ", ")
	env.To = addresses[:j]
	if f.Value == "" {
		f.Problem = `The "To" field is required.`
	}
}

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}
func max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}
