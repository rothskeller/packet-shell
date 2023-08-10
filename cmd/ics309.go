package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/rothskeller/packet-cmd/config"
	"github.com/rothskeller/packet/incident"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(ics309Cmd)
}

var ics309Cmd = &cobra.Command{
	Use:                   "ics309",
	DisableFlagsInUseLine: true,
	Aliases:               []string{"309", "log"},
	Short:                 "Generate and show ICS-309 comms log",
	Long: `The "ics309" command generates an ICS-309 communications log in both CSV and
PDF formats.  It lists all sent and received messages in the current incident
(i.e., current working directory), including receipts.  The generated log is
stored in "ics309.csv" and "ics309.pdf".

After generating the log, the "ics309" command displays the log.  In
interactive (--human) mode, it opens the formatted PDF log in the system PDF
viewer.  In noninteractive (--batch) mode, it sends the log in CSV format to
standard output.

NOTE:  Packet commands automatically remove the saved ICS-309 files after any
change to any message, to avoid reliance on a stale communications log.  Simply
run "ics309" again to generate a new one.
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		// If the generated ICS-309 exists, just show it.
		if term.Human() {
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
		if config.C.IncidentName == "" && term.Human() {
			saveTerm := term
			rootCmd.SetArgs([]string{"edit", "config", "Incident Name"})
			err = rootCmd.Execute()
			term.Close()
			term = saveTerm
			if err != nil {
				return err
			}
		}
		// Generate the file.
		if _, _, err := incident.GenerateICS309(&incident.ICS309Header{
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
		if term.Human() {
			return showICS309()
		} else {
			contents, err := os.ReadFile("ics309.csv")
			if err != nil {
				return err
			}
			os.Stdout.Write(contents)
			return nil
		}
	},
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
