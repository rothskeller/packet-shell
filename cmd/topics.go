package cmd

import (
	"sort"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet/message"
)

const configSlug = `incident/activation/connection settings`
const configHelp = `
Configuration settings for the incident / activation can be viewed with the "show config" command and changed with the "edit config" or "set config" commands.  These commands deal with the following configuration settings:

Operator Call Sign
Operator Name
    These are the FCC call sign and name of the operator running the station. They are used for station identification during the BBS connection, as well as being filled into various forms.
Tactical Call Sign
Tactical Station Name
    These are the assigned call sign and name of the tactical station being operated.  They are filled into various forms.
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
    Internet connections: This is the password to use to log into the BBS.  The "packet" commands will log in using the "Tactical Call Sign" if given, otherwise the "Operator Call Sign".  Note that this password is stored in clear text in the "packet.conf" file; make sure to protect it properly.
Message Numbering
    This is the message number of the first message; subsequent messages will follow the same pattern with increasing sequence numbers.
Default Destination
    This is the address list to be added to the "To" field of any new message.
Default Body Text
    This is text to be added to the body text field of any new message, e.g., "**** This is drill traffic ****".
Incident Name
Activation Number
Operation Start
Operation End
    These are text placed at the top of generated ICS-309 communication logs.
`

const filesSlug = `directory layout and file formats`
const filesHelp = `
Each incident (or other activation) has its own directory.  It contains the following files:

  LOC-111P.txt      ⇥message with local ID "LOC-111P", in RFC-5322 format
  LOC-111P.pdf      ⇥form from "LOC-111P", if any, in PDF format
  LOC-111P.DR#.txt  ⇥delivery receipts for "LOC-111P", if any
  LOC-111P.RR#.txt  ⇥read receipts for "LOC-111P", if any
  REM-222P.txt      ⇥symbolic link: remote ID "REM-222P" to local ID "LOC-111P"
  REM-222P.pdf      ⇥symbolic link: remote ID "REM-222P" to local ID "LOC-111P"
  ics309.csv        ⇥ICS-309 communications log, in CSV format
  ics309.pdf        ⇥ICS-309 communications log, in PDF format
  packet.conf       ⇥incident/activation configuration settings, in JSON format
  packet.log        ⇥text file with log of all BBS communications

For messages that we received, LOC-111P.txt and LOC-111P.pdf contain the received message, LOC-111P.DR0.txt contains the delivery receipt we sent for the message, and REM-222P.txt and REM-222P.pdf are named with the Origin Message ID of the received message.

For messages that we sent, LOC-111P.txt and LOC-111P.pdf contain the sent message, LOC-111P.DR#.txt and LOC-111P.RR#.txt contain the receipts we received for the message, and REM-222P.txt and REM-222P.pdf are named with the destination stations' message IDs for the message we sent (which we pull from their delivery receipts).

For outgoing messages that we haven't sent yet, LOC-111P.txt and LOC-111P.pdf contain the message; none of the other message files exist.  The message has an "X-Packet-Queued: true" header if it is queued to be sent.

PDF files are created only if the program is built with PDF rendering support, and only for messages containing a known form type.
`

const scriptSlug = `how to use "packet" from scripts`
const scriptHelp = `
The "packet" commands provide script-friendly behavior when standard input and output are not a terminal.  In particular:
  - ⇥When standard output is not a terminal:
    - ⇥Confirmation and transient status messages are suppressed.
    - ⇥All colorization of the output is suppressed.
    - ⇥Commands that normally produce tables will produce CSV output.
    - ⇥Error messages are written to standard error instead of standard output.
  - ⇥When either standard input or standard output is not a terminal:
    - ⇥The "new" command prints to standard output the local message ID of the new message, so that it can be used in subsequent commands.
    - ⇥The "new" command does not start an editor, and the "edit" command is not available.
    - ⇥The "set" command reads the new value of a field from standard input without any prompting or editing.
    - ⇥The "set" and "show" commands require the full name (or PIFO tag) of the field being changed or displayed, not an abbreviation.  This prevents scripts from being broken by future additions of fields or by future changes to the abbreviation heuristics.

For a script to send a message, it will usually follow this sequence:
  1. ⇥Run "packet set config" commands to set necessary configuration settings.
  2. ⇥Run "packet new" command to start a new message.
  3. ⇥Run "packet set" commands to set fields of that message.
  4. ⇥Run "packet queue" command to queue the message for sending.
  5. ⇥Run "packet connect" command to connect to the BBS and send it.
`

const typesSlug = `list of supported message types`

func typesHelp() {
	var tags = make([]string, 0, len(message.RegisteredTypes))
	var taglen int
	for tag := range message.RegisteredTypes {
		if msg := message.Create(tag); msg == nil || !msg.Editable() {
			continue
		}
		tags = append(tags, tag)
		aliaslen := len(aliases[tag])
		if aliaslen != 0 {
			aliaslen += 3
		}
		taglen = max(taglen, len(tag)+aliaslen)
	}
	sort.Strings(tags)
	for _, tag := range tags {
		var name = message.RegisteredTypes[tag].Name
		if alias := aliases[tag]; alias != "" {
			tag += " (" + alias + ")"
		}
		cio.ShowNameValue(tag, name, taglen)
	}
	cio.EndNameValueList()
}
