// Package terminal handles all input and output from the packet shell,
// addressing the varying needs of batch and human users and different terminal
// capabilities.
package terminal

import (
	"time"

	"github.com/rothskeller/packet/message"
	"github.com/spf13/cobra"
)

type Terminal interface {
	// Human returns whether output should be formatted for humans.
	Human() bool
	// Confirm prints a confirmation message describing the completion of a
	// requested activity.
	Confirm(f string, args ...any)
	// Status prints a transient status update message, which will be
	// replaced by the next output.
	Status(f string, args ...any)
	// Error prints an error message, suitably styled.
	Error(f string, args ...any)
	// BulletinScheduleTable prints the table of scheduled bulletin checks.
	BulletinScheduleTable()
	// ListMessage displays the message list line for a single message.
	ListMessage(li *ListItem)
	// EndMessageList finalizes the message list, and displays the supplied
	// string if no messages were listed.
	EndMessageList(s string)
	// ShowNameValue displays a name/value pair, such as a configuration
	// setting or a message field, possibly as part of a list of such pairs.
	// When it is part of a list, nameWidth should be set to the width of
	// the longest name in the list.  When it is not, nameWidth should be
	// zero.
	ShowNameValue(name, value string, nameWidth int)
	// EndNameValueList finalizes the name/value list.
	EndNameValueList()
	// EditField presents an editor for a single field.  It returns the
	// result of the editing.  labelWidth is the width of the largest label
	// in a set of fields being edited together; a value of 0 means that
	// only a single field is being edited.
	EditField(field *message.Field, labelWidth int) (EditResult, error)
	// ReadCommand reads a command from the command line, with editing
	// support where possible.
	ReadCommand() (string, error)
	// Close closes the terminal, restoring settings as needed.
	Close()
}

type ListItem struct {
	Handling string // "I", "P", "R", "B" for bulletin
	Flag     string // "DRAFT", "QUEUE", "NO RCPT", "HAVE RCPT"
	Time     time.Time
	From     string
	LMI      string
	To       string
	Subject  string
	NoHeader bool
}

// EditResult is the result of an EditField operation.
type EditResult byte

// Values for EditResult:
const (
	// ResultEOF says that the edit was terminated by reaching end of file
	// on standard input.  (This only happens in batch mode.)
	ResultEOF EditResult = '\004'
	// ResultNext says that the editor should move on to the next field.
	ResultNext EditResult = 'v'
	// ResultPrevious says that the edit should move back to the previous
	// field.
	ResultPrevious EditResult = '^'
	// ResultDone says that the edit should stop.
	ResultDone EditResult = '\033'
)

// New returns a Terminal implementation appropriate for the requested input/
// output mode (--script or --no-script) and attached devices (terminal or not).
func New(cmd *cobra.Command) (t Terminal) {
	// If they asked for script mode, give it to them.
	if script, _ := cmd.Flags().GetBool("script"); script {
		return newBatch()
	}
	// If they asked for no-script mode, give it to them.
	if noscript, _ := cmd.Flags().GetBool("no-script"); noscript {
		if isTerminal {
			return newStyled()
		} else {
			return newPlain()
		}
	}
	// They didn't ask, so decide based on terminal type.
	if isTerminal {
		return newStyled()
	} else {
		return newBatch()
	}
}

var spaces = "                                                                                                                                                                                                                                                                                                            "

// setLength returns the input string, truncated or right-padded to be exactly
// the requested length.
func setLength(s string, l int) string {
	if l < 0 {
		return s
	}
	if len(s) < l {
		return s + spaces[:l-len(s)]
	}
	return s[:l]
}

// setMaxLength returns the input string, truncated if necessary to be no longer
// than the requested length.
func setMaxLength(s string, l int) string {
	if l < 0 {
		return s
	}
	if len(s) > l {
		return s[:l]
	}
	return s
}
