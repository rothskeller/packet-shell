package cio

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/rothskeller/packet/message"
)

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

const editorHelp = `Editor: [F1]=Help [Tab]=Next [Shift-Tab]=Prev [ESC]=Exit [Ctrl-C]=Abort`

func StartEdit() {
	if !InputIsTerm || !OutputIsTerm {
		return
	}
	print(colorHelp, setLength(editorHelp, Width-1))
	print(0, "\n")
}

func EditField(f *message.Field, labelWidth int) (EditResult, error) {
	if !InputIsTerm || !OutputIsTerm {
		return readFieldStdin(f)
	}
	return editField(f, labelWidth)
}
func readFieldStdin(f *message.Field) (EditResult, error) {
	contents, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 0, fmt.Errorf("reading stdin: %s", err)
	}
	contents = bytes.TrimRight(contents, "\r\n")
	contents = bytes.ReplaceAll(contents, []byte{'\r', '\n'}, []byte{'\n'})
	contents = bytes.ReplaceAll(contents, []byte{'\r'}, []byte{'\n'})
	f.EditApply(f, string(contents))
	return ResultEOF, nil
}

type modefunc func() (modefunc, EditResult, error)

type editor struct {
	field      *message.Field
	labelWidth int
	fieldWidth int
	value      string
	choices    []string
	cursor     int
	sels, sele int
	changed    bool
}

func editField(f *message.Field, labelWidth int) (result EditResult, err error) {
	var (
		mode      modefunc
		e         editor
		restarted bool
	)
	rawMode()
	defer restoreTerminal()
RESTART:
	e = editor{
		field:      f,
		labelWidth: max(labelWidth, len(f.Label)),
		fieldWidth: f.EditWidth, // modified below
		value:      f.EditValue(f),
		choices:    f.Choices.ListHuman(),
	}
	e.cursor = len(e.value)
	e.sels, e.sele = 0, e.cursor
	for _, c := range e.choices {
		e.fieldWidth = max(e.fieldWidth, len(c))
	}
	if len(e.choices) != 0 && (e.value == "" || slices.Contains(e.choices, e.value)) {
		mode = e.choicesMode
	} else if labelWidth+2+len(e.value) >= Width || strings.Contains(e.value, "\n") {
		mode = e.multilineMode
	} else {
		mode = e.onelineMode
	}
	for mode != nil && err == nil {
		mode, result, err = mode()
	}
	if err != nil {
		return 0, err
	}
	f.EditApply(f, e.value)
	e.value = f.EditValue(f)
	e.display()
	if problem := f.EditValid(f); problem != "" && result != ResultPrevious && (e.changed || !restarted) {
		Error(problem)
		restarted = true
		goto RESTART
	}
	return result, nil
}

func (e *editor) showHelp() {
	move(0, 0)
	cleanTerminal()
	lines, _ := wrap(e.field.EditHelp, Width-1)
	for _, line := range lines {
		print(colorHelp, setLength(line, Width-1))
		print(0, "\n")
	}
	print(colorHelp, setLength(editorHelp, Width-1))
	print(0, "\n")
	cleanTerminal()
}

func hideValue(s string) string {
	return strings.Repeat("*", len(s))
}
