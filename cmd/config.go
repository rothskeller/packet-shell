package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(configCmd)
}

var configCmd = &cobra.Command{
	Use:                   "configuration",
	DisableFlagsInUseLine: true,
	Aliases:               []string{"configure", "config", "conf", "settings"},
	Short:                 "describes incident/activation/connection settings",
	Long: `Configuration settings for the incident / activation can be viewed with the
"show config" command and changed with the "edit config" or "set config"
commands.  These commands deal with the following configuration settings:

Operator Call Sign
Operator Name
    These are the FCC call sign and name of the operator running the station.
    They are used for station identification during the BBS connection, as
    well as being filled into various forms.
Tactical Call Sign
Tactical Station Name
    These are the assigned call sign and name of the tactical station being
    operated.  They are filled into various forms.
BBS Connection
    This is the type of BBS connection to use:  Radio or Internet.
BBS Address
    Radio connections: This is the AX.25 address of the BBS (e.g. W6XSC-1).
TNC Serial Port
    Radio connections: This the serial port to use to connect to the TNC.
BBS Hostname
    Internet connections: This is the hostname of the BBS server.
BBS Port
    Internet connections: This is the port number to connect to on the BBS.
Password
    Internet connections: This is the password to use to log into the BBS.
    The "packet" commands will log in using the "Tactical Call Sign" if given,
    otherwise the "Operator Call Sign".  Note that this password is stored in
    clear text in the "packet.conf" file; make sure to protect it properly.
Message Numbering
    This is the message number of the first message; subsequent messages will
    follow the same pattern with increasing sequence numbers.
Default Body Text
    This is text to be added to the body text field of any new message, e.g.,
    "**** This is drill traffic ****".
Incident Name
Activation Number
Operation Start
Operation End
    These are text placed at the top of generated ICS-309 communication logs.
`,
}
