package cmd

import (
	"fmt"
	"strings"

	"github.com/rothskeller/packet-cmd/config"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(configCmd)
}

var configCmd = &cobra.Command{
	Use:                   "configure [«name» [=] [«value»]]",
	DisableFlagsInUseLine: true,
	Aliases:               []string{"config", "conf"},
	Short:                 "Configure incident/activation/connection settings",
	Long: `
The "configure" (or "conf" or "config") command sets configuration settings
for the incident/activation and the BBS connection.

With no arguments, it displays the values of all configuration settings.
With only a setting name, it displays the value of that setting.
With a name and value, it sets that setting to that value.
With a name and equals sign, it clears the value of that setting.

Known settings are:
    incident    incident name
    activation  activation number
    period      operational period (MM/DD/YYYY HH:MM [MM/DD/YYYY] HH:MM)
    operator    operator call sign and name
    tactical    tactical station call sign and name
    bbs         BBS to connect to (W6XSC-1 or w6xsc.ampr.org:8080)
    tnc         serial port of TNC
    password    password for logging into BBS
    msgid       starting local message ID (XXX-###P)
    defbody     default body string for new messages
`,
	Args: cobra.MaximumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		args = configCmdArgs(args)
		switch len(args) {
		case 0:
			showAllSettings()
		case 1:
			return showOneSetting(args[0])
		case 2:
			// TODO
			// if err = config.SetSetting(args) {}
		}
		return nil
	},
}

// configCmdArgs rewrites the arguments so that query cases are always one
// argument, assignment cases are always two arguments, and there is never an
// equals sign.
func configCmdArgs(args []string) []string {
	// If the first argument contains an equals sign, split on it.
	if len(args) != 0 {
		if idx := strings.IndexByte(args[0], '='); idx >= 0 {
			args = append([]string{args[0][:idx], "=", args[0][idx+1:]}, args[1:]...)
		}
	}
	// If the second argument is a lone equals sign, remove it, but be sure
	// it is followed by something.
	if len(args) > 1 && args[1] == "=" {
		if len(args) == 2 {
			args[1] = ""
		} else {
			args[1] = strings.Join(args[2:], " ")
		}
	}
	return args
}

func showAllSettings() {
	term.ShowNameValue("incident", config.C.IncidentName, 10)
	term.ShowNameValue("activation", config.C.ActivationNum, 10)
	term.ShowNameValue("period", config.C.OpStartDate+" "+config.C.OpStartTime+" "+config.C.OpEndDate+" "+config.C.OpEndTime, 10)
	term.ShowNameValue("operator", config.C.OpCall+" "+config.C.OpName, 10)
	term.ShowNameValue("tactical", config.C.TacCall+" "+config.C.TacName, 10)
	term.ShowNameValue("bbs", config.C.BBSAddress, 10)
	term.ShowNameValue("tnc", config.C.SerialPort, 10)
	term.ShowNameValue("msgid", config.C.MessageID, 10)
	term.ShowNameValue("defbody", config.C.DefBody, 10)
}

func showOneSetting(name string) error {
	switch name {
	case "incident":
		term.ShowNameValue("incident", config.C.IncidentName, 0)
	case "activation":
		term.ShowNameValue("activation", config.C.ActivationNum, 0)
	case "period":
		term.ShowNameValue("period", config.C.OpStartDate+" "+config.C.OpStartTime+" "+config.C.OpEndDate+" "+config.C.OpEndTime, 0)
	case "operator":
		term.ShowNameValue("operator", config.C.OpCall+" "+config.C.OpName, 0)
	case "tactical":
		term.ShowNameValue("tactical", config.C.TacCall+" "+config.C.TacName, 0)
	case "bbs":
		term.ShowNameValue("bbs", config.C.BBSAddress, 0)
	case "tnc":
		term.ShowNameValue("tnc", config.C.SerialPort, 0)
	case "msgid":
		term.ShowNameValue("msgid", config.C.MessageID, 0)
	case "defbody":
		term.ShowNameValue("defbody", config.C.DefBody, 0)
	default:
		return fmt.Errorf("no such setting %q", name)
	}
	return nil
}
