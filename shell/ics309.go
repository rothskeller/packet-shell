package shell

import (
	_ "embed" // .
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/rothskeller/packet/incident"
)

// cmdICS309 implements the ics309 command.
func cmdICS309(args []string) bool {
	if len(args) != 0 {
		io.WriteString(os.Stderr, "usage: packet ics309\n")
		return false
	}
	// If the generated ICS-309 exists, just show it.
	if _, err := os.Stat("ics309.pdf"); !errors.Is(err, os.ErrNotExist) {
		return showICS309()
	} else if _, err := os.Stat("ics309-p1.pdf"); !errors.Is(err, os.ErrNotExist) {
		return showICS309()
	}
	// Make sure we have the incident settings.
	if !requestConfig("incident", "activation", "period", "tactical", "operator") {
		return false
	}
	// Generate the file.
	if _, _, err := incident.GenerateICS309(&incident.ICS309Header{
		IncidentName:  config.IncidentName,
		ActivationNum: config.ActivationNum,
		OpStartDate:   config.OpStartDate,
		OpStartTime:   config.OpStartTime,
		OpEndDate:     config.OpEndDate,
		OpEndTime:     config.OpEndTime,
		OpCall:        config.OpCall,
		OpName:        config.OpName,
		TacCall:       config.TacCall,
		TacName:       config.TacName,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		return false
	}
	return showICS309()
}

func helpICS309() {
	io.WriteString(os.Stdout, `The "ics309" command generates and displays an ICS-309 log.
    usage: packet ics309
The "ics309" command generates an ICS-309 communications log, in both CSV and
PDF formats.  It lists all sent and received messages in the current incident
(i.e., current working directory), including receipts.  The generated log is
stored in "ics309.csv" and "ics309.pdf".  (If multiple pages are needed, each
page is stored in "ics309-p##.pdf".)  After generating the log, the "ics309"
command opens the formatted PDF version in the system PDF viewer.
    NOTE:  Packet commands automatically remove the saved ICS-309 files after
any change to any message, to avoid reliance on a stale communications log.
Simply run "ics309" again to generate a new one.
`)
}

// showICS309 opens the system PDF viewer to show the generated ICS-309 log.
func showICS309() bool {
	var pages []string

	if _, err := os.Stat("ics309.pdf"); err == nil {
		pages = []string{"ics309.pdf"}
	} else {
		pages, _ = filepath.Glob("ics309-p*.pdf")
	}
	if len(pages) == 0 {
		io.WriteString(os.Stderr, "ERROR: generated ICS-309 PDF files are missing\n")
		return false
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd.exe", append([]string{"/C"}, pages...)...)
	case "darwin":
		cmd = exec.Command("open", pages...)
	default:
		cmd = exec.Command("xdg-open", pages...)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: PDF viewer could not be started: %s\n", err)
		return false
	}
	go func() { cmd.Wait() }()
	return true
}
