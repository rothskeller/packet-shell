// Package terminal handles all input and output from the packet shell,
// addressing the varying needs of batch and human users and different terminal
// capabilities.
package terminal

import (
	"os"
	"time"

	"github.com/rothskeller/packet/message"
	"golang.org/x/term"
)

const (
	colorNormal    = 16*256 + 254  // light grey on black
	colorLabel     = 16*256 + 51   // cyan on black
	colorError     = 16*256 + 202  // red on black
	colorBulletin  = 16*256 + 51   // cyan on black
	colorImmediate = 16*256 + 202  // red on black
	colorPriority  = 16*256 + 226  // yellow on black
	colorWhite     = 16*256 + 231  // bright white on black
	colorAlertBG   = 196*256 + 231 // white on red
	colorWarningBG = 226*256 + 16  // black on yellow
	colorSuccessBG = 28*256 + 231  // white on green
	colorEntry     = 238*256 + 231 // white on gray
	colorSelected  = 254*256 + 16  // black on light grey
	colorHelp      = 30*256 + 254  // light grey on dark green
	colorHint      = 16*256 + 250  // grey on black
)

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

type TableWriter interface {
	// Row takes a set of column color, column value pairs.  There must be
	// the same number of columns as were passed to NewTable.  The column
	// color is bg*256+fg, where bg and fg are between 16 and 255 as
	// understood in ANSI escape codes.  bg and/or fg can also be 0 meaning
	// "terminal's default".
	Row(...any)
	// Close closes the table, and writes the provided message if no table
	// rows were emitted.
	Close(none string)
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

// Init initializes the term subsystem; it must be called before any other
// term.* method.  The human and batch flags come from the command line --human
// and --batch arguments.
func Init(human, batch bool) (t Terminal) {
	// We have four basic possibilities:
	// 1.  Batch mode: intended for scripted use.  Minimalist, predictable
	//     output.  CSV formatting of tables.
	// 2.  Plain mode: human use, but on a terminal that doesn't support
	//     styling.  Aligned and padded tables.
	// 3.  ANSI terminal: human use, on a terminal that accepts ANSI codes.
	// 4.  Windows terminal: human use, in a Windows console window.
	// Each of the four is implemented by a different termhandler.
	if !batch && term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd())) {
		return newStyled()
		// Whether this is ANSI or Windows depends on the build.
	} else if human {
		return newPlain()
	} else {
		return newBatch()
	}
}

var spaces = "                                                                                                                                                                                                                                                                                                            "

func setLength(s string, l int) string {
	if l < 0 {
		return s
	}
	if len(s) < l {
		return s + spaces[:l-len(s)]
	}
	return s[:l]
}

func setMaxLength(s string, l int) string {
	if l < 0 {
		return s
	}
	if len(s) > l {
		return s[:l]
	}
	return s
}
