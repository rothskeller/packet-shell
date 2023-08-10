package terminal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rothskeller/packet-cmd/config"
	"github.com/rothskeller/packet/message"
	"golang.org/x/exp/maps"
)

type plain struct {
	width        int
	listItemSeen bool
}

func newPlain() (t *plain) {
	t = new(plain)
	if w, _ := strconv.Atoi(os.Getenv("COLUMNS")); w > 0 {
		t.width = w
	} else {
		t.width = 80
	}
	return t
}

func (*plain) Human() bool { return true }

func (t *plain) Confirm(f string, args ...any) {
	var s = fmt.Sprintf(f, args...)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	io.WriteString(os.Stdout, s)
}

func (t *plain) Status(f string, args ...any) {
	if f != "" {
		t.Confirm(f, args...)
	}
}

func (t *plain) BulletinScheduleTable() {
	var (
		col1  = []string{"AREA"}
		col2  = []string{"SCHEDULE"}
		col3  = []string{"LAST CHECK"}
		len1  = 4
		len2  = 8
		areas = maps.Keys(config.C.Bulletins)
	)
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
		fmt.Printf("%-*.*s  %-*.*s  %s\n", len1, len1, area, len2, len2, col2[i], col3[i])
	}
}

func (t *plain) ListMessage(li *ListItem) {
	var tstr, fromArrow, toArrow string

	if !t.listItemSeen && !li.NoHeader {
		io.WriteString(os.Stdout, "TIME  FROM        LOCAL ID    TO         SUBJECT\n")
	}
	t.listItemSeen = true
	if !li.Time.IsZero() {
		tstr = li.Time.Format("15:04")
	} else if li.Flag == "QUEUE" || li.Flag == "DRAFT" {
		tstr = li.Flag
	}
	if li.From != "" {
		fromArrow = "→"
	}
	if li.Flag == "NO RCPT" {
		li.From = li.Flag
	}
	if li.To != "" {
		toArrow = "→"
	}
	fmt.Printf("%-.5s %-9.9s %1s %-9.9s %1s %-9.9s  %-.*s\n",
		tstr, li.From, fromArrow, li.LMI, toArrow, li.To, t.width-41, li.Subject)
}

func (t *plain) EndMessageList(s string) {
	if !t.listItemSeen && s != "" {
		io.WriteString(os.Stdout, s)
		io.WriteString(os.Stdout, "\n")
	}
	t.listItemSeen = false
}

func (t *plain) ShowNameValue(name, value string, nameWidth int) {
	var (
		linelen int
		lines   []string
		indent  string
	)
	// Find the length of the longest line in the value.
	nameWidth = max(nameWidth, len(name))
	value = strings.TrimRight(value, "\n")
	lines = strings.Split(value, "\n")
	for _, line := range strings.Split(value, "\n") {
		linelen = max(linelen, len(line))
	}
	// If the longest line fits to the right of the name, show it that way.
	// Otherwise, show it on the following lines with a 4-space indent.
	io.WriteString(os.Stdout, name)
	if linelen <= t.width-nameWidth-3 {
		io.WriteString(os.Stdout, spaces[:nameWidth+2-len(name)])
		indent = spaces[:nameWidth+2]
	} else {
		io.WriteString(os.Stdout, "\n    ")
		lines, _ = wrap(value, t.width-5)
		indent = spaces[:4]
	}
	// Show the lines.
	for i, line := range lines {
		if i != 0 {
			io.WriteString(os.Stdout, indent)
		}
		io.WriteString(os.Stdout, line)
		io.WriteString(os.Stdout, "\n")
	}
}

func (t *plain) EndNameValueList() {}

func (t *plain) Error(f string, args ...any) {
	s := fmt.Sprintf(f, args...)
	fmt.Fprintf(os.Stderr, "ERROR: %s\n", strings.TrimRight(s, "\n"))
}

func (t *plain) EditField(f *message.Field, _ int) (EditResult, error) {
	var (
		restarted  bool
		labelWidth int
		value      string
		mlvalue    bool
		fieldWidth int
		scan       = bufio.NewScanner(os.Stdin)
	)
	labelWidth = len(f.Label)
RESTART:
	value = f.EditValue(f)
	mlvalue = strings.Contains(value, "\n")
	fieldWidth = max(f.EditWidth, len(value))
	for _, c := range f.Choices.ListHuman() {
		fieldWidth = max(fieldWidth, len(c))
	}
	// Write the prompt and current value.
	io.WriteString(os.Stdout, f.Label)
	if (restarted || value == "") && labelWidth+fieldWidth+3 < t.width {
		io.WriteString(os.Stdout, ": ")
	} else if labelWidth+len(value)+fieldWidth+6 < t.width && !mlvalue && !restarted {
		io.WriteString(os.Stdout, " [")
		io.WriteString(os.Stdout, value)
		io.WriteString(os.Stdout, "]: ")
	} else if value != "" && !restarted && labelWidth+len(value)+6 < t.width && !mlvalue {
		io.WriteString(os.Stdout, " [")
		io.WriteString(os.Stdout, value)
		io.WriteString(os.Stdout, "]:\n> ")
	} else {
		io.WriteString(os.Stdout, ":\n")
		if !restarted && value != "" {
			lines, _ := wrap(value, t.width-3)
			linelen := 0
			for _, line := range lines {
				linelen = max(linelen, len(line))
			}
			for _, line := range lines {
				io.WriteString(os.Stdout, "[")
				io.WriteString(os.Stdout, line)
				io.WriteString(os.Stdout, spaces[:linelen-len(line)])
				io.WriteString(os.Stdout, "]\n")
			}
		}
		io.WriteString(os.Stdout, "> ")
	}
	// Read lines of the new value.
	value = ""
	for scan.Scan() {
		var line = scan.Text()
		// There are some special case entries for the first line.
		if value == "" {
			switch line {
			case "": // no change to field
				if problem := f.EditValid(f); problem != "" && !restarted {
					fmt.Printf("ERROR: %s\n", problem)
					restarted = true
					goto RESTART
				}
				return ResultNext, nil
			case "?": // show help
				lines, _ := wrap(f.EditHelp, t.width-1)
				for _, line := range lines {
					io.WriteString(os.Stdout, line)
					io.WriteString(os.Stdout, "\n")
				}
				goto RESTART
			case ".": // exit editor
				return ResultDone, nil
			case "-": // previous field
				return ResultPrevious, nil
			}
		}
		// If the field is not multiline, and the user didn't put a
		// line continuation mark at the end of it, this is the last
		// line of the value.  Also, if the user has entered two empty
		// lines in a row, that is also the last line of the value.
		if (!f.Multiline && !mlvalue && !strings.HasSuffix(line, "\\")) ||
			(line == "" && strings.HasSuffix(value, "\n\n")) {
			f.EditApply(f, strings.TrimRight(value+line, "\n"))
			var problem string
			if problem = f.PresenceValid(); problem == "" {
				problem = f.EditValid(f)
			}
			if problem != "" {
				fmt.Printf("ERROR: %s\n", problem)
				restarted = true
				goto RESTART
			}
			return ResultNext, nil
		}
		// The line we just read is not the last.  Add it to the value
		// we're building (minus any trailing line continuation mark),
		// and write a new prompt.
		mlvalue = true
		value += strings.TrimRight(line, "\\") + "\n"
		io.WriteString(os.Stdout, "> ")
	}
	return 0, scan.Err()
}

func (t *plain) ReadCommand() (string, error) {
	var scan = bufio.NewScanner(os.Stdin)

	io.WriteString(os.Stdout, "packet> ")
	if scan.Scan() {
		return scan.Text(), nil
	}
	if err := scan.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

func fmtDuration(d time.Duration) (s string) {
	if d >= time.Hour {
		var h time.Duration
		h, d = d/time.Hour, d%time.Hour
		s = fmt.Sprintf("%dh", h)
	}
	if d >= time.Minute {
		s += fmt.Sprintf("%dm", d/time.Minute)
	}
	if s == "" {
		s = "1m"
	}
	return s
}
