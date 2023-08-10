package terminal

import (
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rothskeller/packet-cmd/config"
	"github.com/rothskeller/packet/message"

	"golang.org/x/exp/maps"
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

// Key codes used in this program.  This isn't all possible key codes, but it's
// the ones that are relevant to us.
const (
	keyUp = 0x80 + iota
	keyDown
	keyRight
	keyLeft
	keyShiftUp
	keyShiftDown
	keyShiftRight
	keyShiftLeft
	keyCtrlRight
	keyCtrlLeft
	keyCtrlShiftRight
	keyCtrlShiftLeft
	keyHome
	keyEnd
	keyShiftHome
	keyShiftEnd
	keyDelete
	keyF1
	keyBackTab
)

type styled struct {
	width        int
	lastColor    int
	listItemSeen bool
	haveStatus   bool
	x, y         int
	hideCursor   bool
	buf          *screenBuf
}

func newStyled() (t *styled) {
	t = new(styled)
	if w, _, _ := term.GetSize(int(os.Stdout.Fd())); w > 0 {
		t.width = w
	} else if w, _ := strconv.Atoi(os.Getenv("COLUMNS")); w > 0 {
		t.width = w
	} else {
		t.width = 80
	}
	return t
}

func (*styled) Human() bool { return true }

func (t *styled) Close() { cookedMode() }

func (t *styled) Confirm(f string, args ...any) {
	var s = fmt.Sprintf(f, args...)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	t.clearStatus()
	io.WriteString(os.Stdout, s)
}

func (t *styled) Status(f string, args ...any) {
	t.clearStatus()
	if f != "" {
		var s = fmt.Sprintf(f, args...)
		io.WriteString(os.Stdout, strings.TrimRight(s, "\n"))
		t.haveStatus = true
	}
}

func (t *styled) BulletinScheduleTable() {
	var (
		col1  = []string{"AREA"}
		col2  = []string{"SCHEDULE"}
		col3  = []string{"LAST CHECK"}
		len1  = 4
		len2  = 8
		areas = maps.Keys(config.C.Bulletins)
	)
	t.clearStatus()
	if len(areas) == 0 {
		io.WriteString(os.Stdout, "No bulletin checks are scheduled in any area.\n")
		return
	}
	sort.Strings(areas)
	col1 = append(col1, areas...)
	for i, area := range col1 {
		if i == 0 {
			continue
		}
		len1 = max(len1, len(area))
		bc := config.C.Bulletins[area]
		if bc.Frequency == 0 {
			col2 = append(col2, "one time")
		} else {
			col2 = append(col2, "every "+fmtDuration(bc.Frequency))
		}
		len2 = max(len2, len(col2[i]))
		if bc.LastCheck.IsZero() {
			col3 = append(col3, "never")
		} else {
			col3 = append(col3, fmtDuration(time.Since(bc.LastCheck))+" ago")
		}
	}
	for i, area := range col1 {
		var color int
		if i == 0 {
			color = colorWhite
		}
		t.print(color, setLength(area, len1+2))
		t.print(color, setLength(col2[i], len2+2))
		t.print(color, col3[i])
		t.print(0, "\n")
	}
}

func (t *styled) ListMessage(li *ListItem) {
	var lineColor int

	t.clearStatus()
	if !t.listItemSeen && !li.NoHeader {
		t.print(colorWhite, "TIME  FROM        LOCAL ID    TO         SUBJECT")
		t.print(0, "\n")
	}
	t.listItemSeen = true
	switch li.Handling {
	case "B":
		lineColor = colorBulletin
	case "I":
		lineColor = colorImmediate
	case "P":
		lineColor = colorPriority
	}
	if !li.Time.IsZero() {
		t.print(lineColor, li.Time.Format("15:04 "))
	} else if li.Flag == "QUEUE" {
		t.print(colorWarningBG, li.Flag)
		t.print(lineColor, " ")
	} else if li.Flag == "DRAFT" {
		t.print(colorAlertBG, li.Flag)
		t.print(lineColor, " ")
	} else {
		t.print(lineColor, "      ")
	}
	if li.Flag == "NO RCPT" {
		t.print(colorWarningBG, li.Flag)
		t.print(lineColor, "     ")
	} else if li.From != "" {
		t.print(lineColor, setLength(li.From, 9)+" → ")
	} else {
		t.print(lineColor, "            ")
	}
	t.print(lineColor, setLength(li.LMI, 9))
	if li.Flag == "HAVE RCPT" {
		t.print(lineColor, " → ")
		t.print(colorSuccessBG, setMaxLength(li.To, 9))
		if len(li.To) < 9 {
			t.print(lineColor, spaces[:11-len(li.To)])
		} else {
			t.print(lineColor, "  ")
		}
	} else if li.To != "" {
		t.print(lineColor, " → "+setLength(li.To, 9)+"  ")
	} else {
		t.print(lineColor, "              ")
	}
	t.print(lineColor, setMaxLength(li.Subject, t.width-41))
	t.print(0, "\n")
}

func (t *styled) EndMessageList(s string) {
	t.clearStatus()
	if !t.listItemSeen && s != "" {
		t.print(0, s)
		t.print(0, "\n")
	}
	t.listItemSeen = false
}

func (t *styled) ShowNameValue(name, value string, nameWidth int) {
	var (
		linelen int
		lines   []string
		indent  string
	)
	t.clearStatus()
	// Find the length of the longest line in the value.
	nameWidth = max(nameWidth, len(name))
	value = strings.TrimRight(value, "\n")
	lines = strings.Split(value, "\n")
	for _, line := range strings.Split(value, "\n") {
		linelen = max(linelen, len(line))
	}
	// If the longest line fits to the right of the name, show it that way.
	// Otherwise, show it on the following lines with a 4-space indent.
	if linelen <= t.width-nameWidth-3 {
		t.print(colorLabel, setLength(name, nameWidth)+"  ")
		indent = spaces[:nameWidth+2]
	} else {
		t.print(colorLabel, name)
		t.print(0, "\n    ")
		lines, _ = wrap(value, t.width-5)
		indent = spaces[:4]
	}
	// Show the lines.
	for i, line := range lines {
		if i != 0 {
			t.print(0, indent)
		}
		t.print(0, line)
		t.print(0, "\n")
	}
}

func (t *styled) EndNameValueList() {}

func (t *styled) Error(f string, args ...any) {
	t.clearStatus()
	t.print(colorError, "ERROR: ")
	s := fmt.Sprintf(f, args...)
	t.print(colorError, strings.TrimRight(s, "\n"))
	t.print(0, "\n")
}

type modefunc func() (modefunc, EditResult, error)

type editor struct {
	term       *styled
	field      *message.Field
	labelWidth int
	fieldWidth int
	value      string
	choices    []string
	cursor     int
	sels, sele int
	changed    bool
}

func (t *styled) EditField(f *message.Field, labelWidth int) (result EditResult, err error) {
	var (
		mode      modefunc
		e         editor
		restarted bool
	)
	rawMode()
	defer cookedMode()
RESTART:
	e = editor{
		term: t, field: f,
		labelWidth: max(labelWidth, len(f.Label)),
		fieldWidth: f.EditWidth, // modified below
		value:      f.EditValue(f),
		choices:    f.Choices.ListHuman(),
	}
	e.cursor = len(e.value)
	e.sels, e.sele = e.cursor, e.cursor
	for _, c := range e.choices {
		e.fieldWidth = max(e.fieldWidth, len(c))
	}
	if len(e.choices) != 0 && (e.value == "" || slices.Contains(e.choices, e.value)) {
		mode = e.choicesMode
	} else if labelWidth+2+len(e.value) >= t.width || strings.Contains(e.value, "\n") {
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
		t.Error(problem)
		restarted = true
		goto RESTART
	}
	return result, nil
}

func (e *editor) showHelp() {
	e.term.move(0, 0)
	e.term.clearToEOS()
	lines, _ := wrap(e.field.EditHelp, e.term.width-1)
	for _, line := range lines {
		e.term.print(colorHelp, setLength(line, e.term.width-1))
		e.term.print(0, "\n")
	}
	e.term.clearToEOS()
}

// wrap wraps a string to fit within the specified width.  It wraps at
// whitespace where possible, and mid-word if necessary.  The returned offsets
// array gives the offset into the original string of the start of each line.
func wrap(s string, width int) (lines []string, offsets []int) {
	offsets = append(offsets, 0)
	offset := 0

	for _, line := range strings.Split(s, "\n") {
		for len(line) > width {
			if idx := strings.LastIndexByte(line[:width], ' '); idx > 0 {
				lines = append(lines, line[:idx+1])
				line = line[idx+1:]
				offset += idx + 1
				offsets = append(offsets, offset)
			} else {
				lines = append(lines, line[:width])
				line = line[width:]
				offset += width
				offsets = append(offsets, width)
			}
		}
		lines = append(lines, line)
		offset += len(line) + 1 // account for newline
		offsets = append(offsets, offset)
	}
	// Note that this leaves the final offset at len(s)+1, since we
	// accounted for a newline on the final line that isn't actually in the
	// string.  However, this is good, because it means cursorToXY(len(s))
	// will return the end of the last line rather than the start of the
	// nonexistent one.
	return lines, offsets
}
