package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet-shell/config"
	"github.com/rothskeller/packet/incident"

	"github.com/spf13/pflag"
)

const (
	ics309Slug = `Generate and show ICS-309 comms log`
	ics309Help = `
usage: packet ics309

The "ics309" (or "309") command generates an ICS-309 communications log in both CSV format and, if PDF rendering support has been built into the program, PDF format.  It lists all sent and received messages in the current incident (i.e., current working directory), including receipts.  The generated log is stored in "ics309.csv" and "ics309.pdf".

After generating the log, the "ics309" command displays the log.  If standard output is a terminal, the log is opened in PDF format in the system PDF viewer.  Otherwise, the log is sent in CSV format to standard output.

NOTE:  Packet commands automatically remove the saved ICS-309 files after any change to any message, to avoid reliance on a stale communications log.  Simply run "ics309" again to generate a new one.
`
)

func cmdICS309(args []string) (err error) {
	flags := pflag.NewFlagSet("ics309", pflag.ContinueOnError)
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"ics309"})
	} else if err != nil {
		cio.Error("%s", err.Error())
		return usage(ics309Help)
	}
	if len(args) != 0 {
		return usage(ics309Help)
	}
	// If the generated ICS-309 exists, just show it.
	if cio.OutputIsTerm {
		if _, err := os.Stat("ics309.pdf"); !errors.Is(err, os.ErrNotExist) {
			return showICS309()
		}
	} else {
		if contents, err := os.ReadFile("ics309.csv"); !errors.Is(err, os.ErrNotExist) {
			os.Stdout.Write(contents)
			return nil
		}
	}
	// Make sure we have the incident settings.
	if config.C.IncidentName == "" && cio.InputIsTerm && cio.OutputIsTerm {
		if err = run([]string{"edit", "config", "Incident Name"}); err != nil {
			return err
		}
	}
	// Generate the file.
	if err := incident.GenerateICS309(&incident.ICS309Header{
		IncidentName:  config.C.IncidentName,
		ActivationNum: config.C.ActivationNum,
		OpStartDate:   config.C.OpStartDate,
		OpStartTime:   config.C.OpStartTime,
		OpEndDate:     config.C.OpEndDate,
		OpEndTime:     config.C.OpEndTime,
		OpCall:        config.C.OpCall,
		OpName:        config.C.OpName,
		TacCall:       config.C.TacCall,
		TacName:       config.C.TacName,
	}); err != nil {
		return fmt.Errorf("generating ICS-309: %s", err)
	}
	if cio.OutputIsTerm {
		return showICS309()
	} else {
		contents, err := os.ReadFile("ics309.csv")
		if err != nil {
			return err
		}
		os.Stdout.Write(contents)
		return nil
	}
}

// showICS309 opens the system PDF viewer to show the generated ICS-309 log.
func showICS309() (err error) {
	if _, err := os.Stat("ics309.pdf"); err != nil {
		return errors.New("generated ICS-309 PDF files are missing")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd.exe", "/C", "ics309.pdf")
	case "darwin":
		cmd = exec.Command("open", "ics309.pdf")
	default:
		cmd = exec.Command("xdg-open", "ics309.pdf")
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting PDF viewer: %s", err)
	}
	go func() { cmd.Wait() }()
	return nil
}
