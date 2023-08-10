package terminal

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rothskeller/packet-cmd/config"
	"github.com/rothskeller/packet/message"
	"golang.org/x/exp/maps"
)

type batch struct {
	cw *csv.Writer
}

func newBatch() *batch {
	return new(batch)
}

func (*batch) Human() bool { return false }

func (t *batch) Confirm(string, ...any) {
	// We don't print confirmations in batch mode.
}

func (t *batch) Status(string, ...any) {
	// We don't print status updates in batch mode.
}

func (t *batch) ListMessage(li *ListItem) {
	if t.cw == nil {
		t.cw = csv.NewWriter(os.Stdout)
		if !li.NoHeader {
			t.cw.Write([]string{"FLAG", "TIME", "FROM", "LMI", "TO", "SUBJECT"})
		}
	}
	var tstr string
	if !li.Time.IsZero() {
		tstr = li.Time.Format(time.RFC3339)
	}
	t.cw.Write([]string{li.Flag, tstr, li.From, li.LMI, li.To, li.Subject})
}

func (t *batch) EndMessageList(string) {
	if t.cw != nil {
		t.cw.Flush()
		t.cw = nil
	}
}

func (t *batch) BulletinScheduleTable() {
	var (
		cw    *csv.Writer
		areas = maps.Keys(config.C.Bulletins)
	)
	if len(areas) == 0 {
		return
	}
	sort.Strings(areas)
	cw = csv.NewWriter(os.Stdout)
	cw.Write([]string{"AREA", "SCHEDULE", "LAST CHECK"})
	for _, area := range areas {
		bc := config.C.Bulletins[area]
		fd := bc.Frequency.String()
		var lc string
		if !bc.LastCheck.IsZero() {
			lc = bc.LastCheck.Format(time.RFC3339)
		}
		cw.Write([]string{area, fd, lc})
	}
	cw.Flush()
}

func (t *batch) ShowNameValue(name, value string, nameWidth int) {
	if nameWidth != 0 {
		if t.cw == nil {
			t.cw = csv.NewWriter(os.Stdout)
		}
		t.cw.Write([]string{name, value})
	} else {
		io.WriteString(os.Stdout, value)
		io.WriteString(os.Stdout, "\n")
	}
}

func (t *batch) EndNameValueList() {
	if t.cw != nil {
		t.cw.Flush()
		t.cw = nil
	}
}

func (t *batch) Error(f string, args ...any) {
	s := fmt.Sprintf(f, args...)
	fmt.Fprintf(os.Stderr, "ERROR: %s\n", strings.TrimRight(s, "\n"))
}

func (t *batch) EditField(f *message.Field, _ int) (EditResult, error) {
	contents, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 0, fmt.Errorf("reading stdin: %s", err)
	}
	f.EditApply(f, string(contents))
	return ResultEOF, nil
}

func (t *batch) ReadCommand() (string, error) {
	return "", errors.New("cannot read commands in --script mode")
}
