// Package editfield provides an ANSI-terminal-based editor for a field value.
package editfield

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

const (
	clearEOL         = "\033[K"
	entryStyle       = "\033[38;5;231;48;5;238m"
	errorStyle       = "\033[38;5;203m"
	fieldLabelStyle  = "\033[38;5;51m"
	helpStyle        = "\033[38;5;254;48;5;30m"
	hideCursor       = "\033[?25l"
	hintStyle        = "\033[38;5;250;48;5;16m"
	newline          = "\r\n"
	newlineIndent    = "\r\n    "
	normalStyle      = "\033[38;5;254;48;5;16m"
	resetAndClearEOS = "\r\033[0;38;5;254;48;5;16m\033[J\033[?25h"
	selectedStyle    = "\033[38;5;16;48;5;254m"
	showCursor       = "\033[?25h"
	spaces           = "                                                                                                              "
)

// NewEditor creates a new editor.  labelWidth is the width of the field label
// column, which should be the length of the longest field label that will be
// used in the editor.  It can be zero if field labels and values don't need to
// line up (e.g., if only one field will be edited or displayed).
func NewEditor(labelWidth int) (e *Editor) {
	e = &Editor{labelWidth: labelWidth}
	e.getScreenWidth()
	return e
}

// An Editor handles a sequence of edit and display requests for related fields.
type Editor struct {
	// for all Display and Edit calls:
	screenWidth int
	labelWidth  int
	// for the current Edit call:
	label      string
	fieldWidth int
	multiline  bool
	value      string
	choices    []string
	help       string
	hint       string
	cursor     int
	x, y       int
	sels, sele int
	changed    bool
}

// Result is the result of an Edit operation.
type Result byte

// Values for Result:
const (
	// ResultError says that the edit could not function due to a terminal
	// error.  The newvalue returned should not be applied to the field.
	ResultError Result = 0
	// ResultEnter says that the edit was terminated by the user pressing
	// the Enter key.
	ResultEnter Result = '\r'
	// ResultTab says that the edit was terminated by the user pressing the
	// Tab key.
	ResultTab Result = '\t'
	// ResultBackTab says that the edit was terminated by the user pressing
	// the BackTab key.
	ResultBackTab Result = '^'
	// ResultESC says that the edit was terminated by the user pressing the
	// ESC key.
	ResultESC Result = '\033'
	// ResultCtrlC says that the edit was terminated by the user pressing
	// Ctrl-C.  The newvalue returned should not be applied to the field.
	ResultCtrlC Result = '\003'
)

// Edit presents a field for editing and returns the edited value.  It expects
// that stdin/stdout is a terminal in raw mode.  "label" is the field label.
// "value" is the initial value of the field.  "width" is the expected maximum
// width of the field value (not enforced).  "choices" is an optional list of
// values that should be easy to enter, e.g. the allowed values for the field.
// "multiline" is true for fields that expect newlines in the value.  "hint" is
// a string displayed when the field is empty to provide guidance for what to
// put in it.
//
// Edit returns the new value for the field, a flag indicating whether the user
// made any changes (even if they later reversed them), and a result code
// indicating how the editing ended.  It leaves the cursor in the same place
// where it was when the edit began, modulo any scrolling: in other words, on
// the first character of the field label.  Generally it should be followed by a
// call to Display.
func (e *Editor) Edit(
	label, value, help, hint string, width int, choices []string, multiline bool,
) (nvalue string, changed bool, result Result) {
	var mode modefunc

	e.cursor = len(value)
	e.label, e.value, e.help, e.hint, e.fieldWidth, e.choices, e.multiline =
		label, value, help, hint, width, choices, multiline
	e.sels, e.sele, e.changed = e.cursor, e.cursor, false
	if len(choices) != 0 && (value == "" || inList(choices, value)) {
		mode = e.choicesMode
	} else if e.labelWidth+2+len(value) >= e.screenWidth || strings.IndexByte(value, '\n') >= 0 {
		mode = e.multilineMode
	} else {
		mode = e.onelineMode
	}
	for mode != nil {
		mode, result = mode()
	}
	return e.value, e.changed, result
}

type modefunc func() (modefunc, Result)

// getScreenWidth gets the width of the screen.  It tries getting it from the
// terminal first; failing that, the $COLUMNS environment variable; failing
// that, it assumes a width of 80.
func (e *Editor) getScreenWidth() {
	var err error

	if e.screenWidth, _, err = term.GetSize(int(os.Stdin.Fd())); err != nil || e.screenWidth <= 0 {
		if e.screenWidth, err = strconv.Atoi(os.Getenv("COLUMNS")); err != nil || e.screenWidth <= 0 {
			e.screenWidth = 80
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func inList[T comparable](list []T, item T) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}
