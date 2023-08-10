package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rothskeller/packet/message"
)

var (
	fccCallSignRE = regexp.MustCompile(`(?i)^(?:A[A-L][0-9][A-Z]{1,3}|[KNW][0-9][A-Z]{2,3}|[KNW][A-Z][0-9][A-Z]{1,3})$`)
	ax25RE        = regexp.MustCompile(`(?i)^(?:A[A-L][0-9][A-Z]{1,3}|[KNW][0-9][A-Z]{2,3}|[KNW][A-Z][0-9][A-Z]{1,3})-(?:1[0-5]|[1-9])$`)
	comPortRE     = regexp.MustCompile(`(?i)COM[1-9]:?$`)
	MsgIDRE       = regexp.MustCompile(`^([0-9][A-Z]{2}|[A-Z][A-Z0-9]{2})-([0-9]*[1-9][0-9]*)([A-Z]?)$`)
)

// packetConf is the name of the packet configuration file.
const packetConf = "packet.conf"

// packetDefaults is the name of the defaults file, stored in the user's HOME.
const packetDefaults = ".packet"

type PacketConfig struct {
	message.BaseMessage `json:"-"`
	IncidentName        string                     `json:",omitempty"`
	ActivationNum       string                     `json:",omitempty"`
	ActNumRequested     bool                       `json:",omitempty"`
	OpStartDate         string                     `json:",omitempty"`
	OpStartTime         string                     `json:",omitempty"`
	OpEndDate           string                     `json:",omitempty"`
	OpEndTime           string                     `json:",omitempty"`
	BBS                 string                     `json:",omitempty"`
	BBSAddress          string                     `json:",omitempty"`
	SerialPort          string                     `json:",omitempty"`
	OpCall              string                     `json:",omitempty"`
	OpName              string                     `json:",omitempty"`
	TacCall             string                     `json:",omitempty"`
	TacName             string                     `json:",omitempty"`
	TacRequested        bool                       `json:",omitempty"`
	Password            string                     `json:",omitempty"`
	MessageID           string                     `json:",omitempty"`
	DefBody             string                     `json:",omitempty"`
	Bulletins           map[string]*BulletinConfig `json:",omitempty"`
	connType            string
	ax25addr            string
	hostname            string
	port                string
}
type BulletinConfig struct {
	Frequency time.Duration
	LastCheck time.Time `json:",omitempty"`
}

// C expresses the session configuration.
var C PacketConfig

func init() {
	C.SerialPort = guessSerialPort()
	// The last configuration saved for any session was also saved to
	// $HOME/.packet.  Read that, if it exists, to override the above
	// defaults and add additional defaults for BBS, Address, OpCall,
	// OpName, and Password.
	if home := os.Getenv("HOME"); home != "" {
		readConfig(filepath.Join(home, packetDefaults))
	}
	// Then read the config in the local directory, if any, to override
	// those.
	readConfig(packetConf)
	// Prepare the fake "message" for editing/showing the configuration.
	C.BaseMessage.Type = &message.Type{
		Tag:     "CONFIG",
		Name:    "packet incident configuration",
		Article: "a",
	}
	C.BaseMessage.Fields = makeConfigFields()
}

// guessSerialPort makes a swag at the device file for the serial port connected
// to the TNC.  It may very well be wrong and there's no attempt to verify it.
func guessSerialPort() (port string) {
	var names []string

	if runtime.GOOS == "windows" {
		// The TNC could be attached to any COM port, and there's no way
		// to know which.  We'll guess COM3 just because it's a fairly
		// common answer.
		return "COM3"
	}
	// On any other OS, look for a /dev/tty* file with USB in the name.
	// Take the highest such name found, since the TNC was probably plugged
	// in later than other devices.
	names, _ = filepath.Glob("/dev/tty*usb*")
	for _, name := range names {
		if name > port {
			port = name
		}
	}
	names, _ = filepath.Glob("/dev/tty*USB*")
	for _, name := range names {
		if name > port {
			port = name
		}
	}
	return port
}

// readConfig reads configuration data from a single file, overlaying what's
// already in the config variable.
func readConfig(filename string) {
	var (
		fh  *os.File
		err error
	)
	if fh, err = os.Open(filename); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "ERROR: %s", err)
		}
		return
	}
	defer fh.Close()
	if err = json.NewDecoder(fh).Decode(&C); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s: %s", filename, err)
		return
	}
}

// SaveConfig saves the configuration.
func SaveConfig() {
	var (
		by   []byte
		home string
		err  error
	)
	if by, err = json.Marshal(&C); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s: %s", packetConf, err)
		return
	}
	if err = os.WriteFile(packetConf, by, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s", err)
	}
	// Also save a reduced version of it to $HOME/.packet to use as defaults
	// for future sessions.
	if home = os.Getenv("HOME"); home == "" {
		return
	}
	reduced := C
	reduced.IncidentName = ""
	reduced.ActivationNum = ""
	reduced.ActNumRequested = false
	reduced.OpStartDate = ""
	reduced.OpStartTime = ""
	reduced.OpEndDate = ""
	reduced.OpEndTime = ""
	reduced.TacCall = ""
	reduced.TacName = ""
	reduced.TacRequested = false
	reduced.MessageID = ""
	reduced.DefBody = ""
	reduced.Bulletins = nil
	by, _ = json.Marshal(&reduced)
	if err = os.WriteFile(filepath.Join(home, packetDefaults), by, 0666); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s", err)
	}
}

func makeConfigFields() []*message.Field {
	if strings.Contains(C.BBSAddress, ":") {
		C.connType, C.ax25addr = "Internet", ""
		p := strings.SplitN(C.BBSAddress, ":", 2)
		C.hostname = p[0]
		if len(p) == 2 {
			C.port = p[1]
		} else {
			C.port = ""
		}
	} else if C.BBSAddress != "" {
		C.connType, C.ax25addr, C.hostname, C.port = "Radio", C.BBSAddress, "", ""
	} else {
		C.connType, C.ax25addr, C.hostname, C.port = "", "", "", ""
	}
	return []*message.Field{
		message.NewFCCCallSignField(&message.Field{
			Label:    "Operator Call Sign",
			Value:    &C.OpCall,
			Presence: message.Required,
			EditHelp: `This is the FCC call sign of the operator using the "packet" command.  It will be sent as identification during BBS connections, and also filled into various forms.  It is required.`,
		}),
		message.NewTextField(&message.Field{
			Label:    "Operator Name",
			Value:    &C.OpName,
			EditHelp: `This is the name of the operator using the "packet" command.  It is filled into various forms.`,
		}),
		message.NewTacticalCallSignField(&message.Field{
			Label:    "Tactical Call Sign",
			Value:    &C.TacCall,
			EditHelp: `This is the call sign of the tactical station being operated.  It is filled into various forms.`,
		}),
		message.NewTextField(&message.Field{
			Label: "Tactical Station Name",
			Value: &C.TacName,
			Presence: func() (message.Presence, string) {
				if C.TacCall == "" {
					return message.PresenceNotAllowed, `unless a "Tactical Call Sign" is provided`
				} else {
					return message.PresenceOptional, ""
				}
			},
			EditHelp: `This is the name of the tactical station being operated.  It is filled into various forms.`,
		}),
		message.NewRestrictedField(&message.Field{
			Label:      "BBS Connnection",
			Value:      &C.connType,
			Choices:    message.Choices{"Radio", "Internet"},
			Presence:   message.Required,
			TableValue: message.TableOmit,
			EditHelp:   `This specifies how the "packet" command will connect to the BBS.  "Radio" means connecting to the BBS over the air, by way of a Kantronics KPC-3 Plus or compatible TNC connected to a radio transceiver.  "Internet" means connecting to the BBS over the Internet.  The choice is required.`,
		}),
		message.NewTextField(&message.Field{
			Label: "BBS Address",
			Value: &C.ax25addr,
			Presence: func() (message.Presence, string) {
				if C.connType == "Radio" {
					return message.PresenceRequired, `when the "BBS Connection" is "Radio"`
				} else {
					return message.PresenceNotAllowed, `unless the "BBS Connection" is "Radio"`
				}
			},
			EditHint: "e.g. W6XSC-1",
			EditHelp: `This is the AX.25 address of the BBS.  It must be an FCC call sign followed by a hyphen and an integer (usually 1).  It is required when the "BBS Connection" is "Radio".`,
			EditApply: func(f *message.Field, s string) {
				C.ax25addr = strings.ToUpper(strings.TrimSpace(s))
				C.BBSAddress = C.ax25addr
				if idx := strings.IndexByte(C.ax25addr, '-'); idx >= 0 {
					C.BBS = C.ax25addr[:idx]
				} else {
					C.BBS = ""
				}
			},
			EditValid: func(f *message.Field) string {
				if p := f.PresenceValid(); p != "" {
					return p
				}
				if C.ax25addr != "" && !ax25RE.MatchString(C.ax25addr) {
					return `The "BBS Address" field does not contain a valid AX.25 address.`
				}
				return ""
			},
		}),
		message.NewTextField(&message.Field{
			Label: "TNC Serial Port",
			Value: &C.SerialPort,
			Presence: func() (message.Presence, string) {
				if C.connType == "Radio" {
					return message.PresenceRequired, `when the "BBS Connection" is "Radio"`
				} else {
					return message.PresenceOptional, ""
				}
			},
			TableValue: message.TableOmit,
			EditHelp:   `This is the serial port for communications with the TNC.  On Windows, this will be COM#, where # is some number.  On other systems, this will be a filename of a character device file in /dev.  It is required when the "BBS Connection" is "Radio".`,
			EditApply: func(f *message.Field, s string) {
				s = strings.TrimSpace(s)
				if runtime.GOOS == "windows" {
					s = strings.ToUpper(s)
				}
				C.SerialPort = s
			},
			EditValid: func(f *message.Field) string {
				if p := f.PresenceValid(); p != "" {
					return p
				}
				if runtime.GOOS == "windows" {
					if !comPortRE.MatchString(C.SerialPort) {
						return `The "TNC Serial Port" field does not contain a valid serial port name (COM#).`
					}
				} else {
					if info, err := os.Stat(C.SerialPort); err != nil || info.Mode().Type()&os.ModeCharDevice == 0 {
						return `The "TNC Serial Port" field does not contain a valid serial port name.`
					}
				}
				return ""
			},
			EditSkip: func(f *message.Field) bool {
				return C.connType != "Radio"
			},
		}),
		message.NewTextField(&message.Field{
			Label: "BBS Hostname",
			Value: &C.hostname,
			Presence: func() (message.Presence, string) {
				if C.connType == "Internet" {
					return message.PresenceRequired, `when the "BBS Connection" is "Internet"`
				} else {
					return message.PresenceNotAllowed, `unless the "BBS Connection" is "Internet"`
				}
			},
			TableValue: message.TableOmit,
			EditHelp:   `This is the Internet hostname of the BBS.  It must start with the FCC call sign of the BBS.  It is required when the "BBS Connection" is "Internet".`,
			EditApply: func(f *message.Field, s string) {
				C.BBSAddress = C.hostname + ":" + C.port
				if idx := strings.IndexFunc(C.hostname, func(r rune) bool {
					return (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
				}); idx >= 0 {
					C.BBS = strings.ToUpper(C.hostname[:idx])
				} else {
					C.BBS = ""
				}
			},
			EditValid: func(f *message.Field) string {
				if p := f.PresenceValid(); p != "" {
					return p
				}
				idx := strings.IndexFunc(C.hostname, func(r rune) bool {
					return (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
				})
				if idx < 0 || !fccCallSignRE.MatchString(C.hostname[:idx]) {
					return `The "BBS Hostname" field does not contain a valid BBS hostname (does not start with an FCC call sign).`
				}
				if host, _, err := net.SplitHostPort(C.hostname + ":1"); err != nil || host != C.hostname {
					return `The "BBS Hostname" field does not contain a valid BBS hostname.`
				}
				return ""
			},
		}),
		message.NewCardinalNumberField(&message.Field{
			Label: "BBS Port Number",
			Value: &C.port,
			Presence: func() (message.Presence, string) {
				if C.connType == "Internet" {
					return message.PresenceRequired, `when the "BBS Connection" is "Internet"`
				} else {
					return message.PresenceNotAllowed, `unless the "BBS Connection" is "Internet"`
				}
			},
			TableValue: message.TableOmit,
			EditHelp:   `This is the TCP/IP port number for the Internet connection to the BBS server.  It is a number between 1024 and 65535.  It is required when the "BBS Connection" is "Internet".`,
			EditApply: func(f *message.Field, s string) {
				C.BBSAddress = C.hostname + ":" + C.port
				if idx := strings.IndexFunc(C.hostname, func(r rune) bool {
					return (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
				}); idx >= 0 {
					C.BBS = strings.ToUpper(C.hostname[:idx])
				} else {
					C.BBS = ""
				}
			},
			EditValid: func(f *message.Field) string {
				if p := f.PresenceValid(); p != "" {
					return p
				}
				if n, err := strconv.Atoi(C.port); C.port != "" && (err != nil || n < 1024 || n > 65535) {
					return `The "BBS Port Number" field does not contain a valid Internet port number.`
				}
				return ""
			},
		}),
		message.NewAggregatorField(&message.Field{
			Label: "BBS Address",
			TableValue: func(f *message.Field) string {
				if C.connType == "Internet" {
					return message.SmartJoin(C.hostname, "port "+C.port, " ")
				}
				return ""
			},
		}),
		message.NewTextField(&message.Field{
			Label: "BBS Password",
			Value: &C.Password,
			Presence: func() (message.Presence, string) {
				if C.connType == "Internet" {
					return message.PresenceRequired, `when the "BBS Connection" is "Internet"`
				} else {
					return message.PresenceNotAllowed, `unless the "BBS Connection" is "Internet"`
				}
			},
			TableValue: message.TableOmit,
			EditHelp:   `This is the password for logging into the BBS server using the "Tactical Call Sign" (if provided) or "Operator Call Sign".  It is required when the "BBS Connection" is "Internet".  Note that this password will be saved in clear text in the local "packet.conf" file; make sure to protect it appropriately.`,
		}),
		message.NewMessageNumberField(&message.Field{
			Label:    "Message Numbering",
			Value:    &C.MessageID,
			EditHelp: `This is the starting message ID.  Newly received and created messages will be assigned message IDs like this one, with increasing sequence numbers.  The message ID must have the form XXX-###P.  For tactical stations, XXX is the three-character message ID prefix assigned to the station.  For personal stations, XXX should be the last three characters of your call sign.  ### is a number (any number of digits).  P is a suffix character, usually "P" but sometimes "M".`,
			EditValue: func(f *message.Field) string {
				if C.MessageID == "" {
					if len(C.TacCall) >= 3 {
						C.MessageID = C.TacCall[:3] + "-100P"
					} else if C.TacCall == "" && len(C.OpCall) >= 3 {
						C.MessageID = C.OpCall[len(C.OpCall)-3:] + "-100P"
					}
				}
				return C.MessageID
			},
		}),
		message.NewMultilineField(&message.Field{
			Label:    "Default Body Text",
			Value:    &C.DefBody,
			EditHelp: `This is optional text to be placed in the most prominent body text field of every new message.  It is primarily used for adding messages like "**** This is drill traffic ****" to all messages during a drill.`,
		}),
		message.NewTextField(&message.Field{
			Label:    "Incident Name",
			Value:    &C.IncidentName,
			EditHelp: `This is the name of the incident or activation for which messages are being handled.  It is put on the top of the generated ICS-309 communications log.`,
		}),
		message.NewTextField(&message.Field{
			Label:    "Activation Number",
			Value:    &C.ActivationNum,
			EditHelp: `This is the activation number assigned to the incident or activation for which messages are being handled.  It is put on the top of the generated ICS-309 communications log.`,
		}),
		message.NewDateTimeField(&message.Field{
			Label:    "Operation Start",
			EditHelp: "This is the date and time when the operational period begins, in MM/DD/YYYY HH:MM format (24-hour clock).  It is put on the top of the generated ICS-309 communications log.",
		}, &C.OpStartDate, &C.OpStartTime),
		message.NewDateTimeField(&message.Field{
			Label:    "Operation End",
			EditHelp: "This is the date and time when the operational period ends, in MM/DD/YYYY HH:MM format (24-hour clock).  It is put on the top of the generated ICS-309 communications log.",
			EditValue: func(f *message.Field) string {
				if C.OpStartDate != "" && C.OpEndDate == "" {
					C.OpEndDate = C.OpStartDate
				}
				return message.SmartJoin(C.OpEndDate, C.OpEndTime, " ")
			},
		}, &C.OpEndDate, &C.OpEndTime),
	}
}
