package shell

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rothskeller/packet/incident"
)

// packetConf is the name of the packet configuration file.
const packetConf = "packet.conf"

// packetDefaults is the name of the defaults file, stored in the user's HOME.
const packetDefaults = ".packet"

type packetConfig struct {
	IncidentName    string                     `json:",omitempty"`
	ActivationNum   string                     `json:",omitempty"`
	ActNumRequested bool                       `json:",omitempty"`
	OpStartDate     string                     `json:",omitempty"`
	OpStartTime     string                     `json:",omitempty"`
	OpEndDate       string                     `json:",omitempty"`
	OpEndTime       string                     `json:",omitempty"`
	BBS             string                     `json:",omitempty"`
	BBSAddress      string                     `json:",omitempty"`
	SerialPort      string                     `json:",omitempty"`
	OpCall          string                     `json:",omitempty"`
	OpName          string                     `json:",omitempty"`
	TacCall         string                     `json:",omitempty"`
	TacName         string                     `json:",omitempty"`
	TacRequested    bool                       `json:",omitempty"`
	Password        string                     `json:",omitempty"`
	MessageID       string                     `json:",omitempty"`
	DefBody         string                     `json:",omitempty"`
	Bulletins       map[string]*bulletinConfig `json:",omitempty"`
}
type bulletinConfig struct {
	Frequency time.Duration
	LastCheck time.Time `json:",omitempty"`
}

// C expresses the session configuration.
var config packetConfig

func init() {
	config.SerialPort = guessSerialPort()
	// The last configuration saved for any session was also saved to
	// $HOME/.packet.  Read that, if it exists, to override the above
	// defaults and add additional defaults for BBS, Address, OpCall,
	// OpName, Password, and BulletinAreas.
	if home := os.Getenv("HOME"); home != "" {
		readConfig(filepath.Join(home, packetDefaults))
	}
	// Then read the config in the local directory, if any, to override
	// those.
	readConfig(packetConf)
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
	if err = json.NewDecoder(fh).Decode(&config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s: %s", filename, err)
		return
	}
}

// saveConfig saves the configuration.
func saveConfig() {
	var (
		by   []byte
		home string
		err  error
	)
	if by, err = json.Marshal(&config); err != nil {
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
	reduced := config
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
	reduced.Bulletins = make(map[string]*bulletinConfig, len(config.Bulletins))
	for area, bc := range config.Bulletins {
		reduced.Bulletins[area] = &bulletinConfig{Frequency: bc.Frequency}
	}
	by, _ = json.Marshal(&reduced)
	if err = os.WriteFile(filepath.Join(home, packetDefaults), by, 0666); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s", err)
	}
}

func cmdSet(args []string) bool {
	switch len(args) {
	case 0:
		showAllSettings()
		return true
	case 1:
		if idx := strings.IndexByte(args[0], '='); idx > 0 {
			return setSetting([]string{args[0][:idx], args[0][idx+1:]})
		}
		return showSetting(args[0])
	default:
		return setSetting(args)
	}
}

func showAllSettings() {
	fmt.Printf(`incident   = %s
activation = %s
period     = %s %s %s %s
operator   = %s %s
tactical   = %s %s
bbs        = %s
tnc        = %s
msgid      = %s
defbody    = %s
`,
		config.IncidentName, config.ActivationNum, config.OpStartDate, config.OpStartTime, config.OpEndDate,
		config.OpEndTime, config.OpCall, config.OpName, config.TacCall, config.TacName, config.BBSAddress,
		config.SerialPort, config.MessageID, config.DefBody)
}

func showSetting(name string) bool {
	switch name {
	case "incident":
		fmt.Printf("incident   = %s\n", config.IncidentName)
	case "activation":
		fmt.Printf("activation = %s\n", config.ActivationNum)
	case "period":
		fmt.Printf("period     = %s %s %s %s\n", config.OpStartDate, config.OpStartTime, config.OpEndDate, config.OpEndTime)
	case "operator":
		fmt.Printf("operator   = %s %s\n", config.OpCall, config.OpName)
	case "tactical":
		fmt.Printf("tactical   = %s %s\n", config.TacCall, config.TacName)
	case "bbs":
		fmt.Printf("bbs        = %s\n", config.BBSAddress)
	case "tnc":
		fmt.Printf("tnc        = %s\n", config.SerialPort)
	case "msgid":
		fmt.Printf("msgid      = %s\n", config.MessageID)
	case "defbody":
		fmt.Printf("defbody    = %s\n", config.DefBody)
	default:
		fmt.Fprintf(os.Stderr, "ERROR: no such setting %q\n", name)
		return false
	}
	return true
}

var (
	fccCallSignRE = regexp.MustCompile(`(?i)^(?:A[A-L][0-9][A-Z]{1,3}|[KNW][0-9][A-Z]{2,3}|[KNW][A-Z][0-9][A-Z]{1,3})$`)
	tacCallSignRE = regexp.MustCompile(`(?i)^[A-Z][A-Z0-9]{5}$`)
	ax25RE        = regexp.MustCompile(`(?i)^(?:A[A-L][0-9][A-Z]{1,3}|[KNW][0-9][A-Z]{2,3}|[KNW][A-Z][0-9][A-Z]{1,3})-(?:1[0-5]|[1-9])$`)
	comPortRE     = regexp.MustCompile(`(?i)COM[1-9]:?$`)
)

func setSetting(args []string) bool {
	var name = args[0]

	args = args[1:]
	if len(args) != 0 && args[0] == "=" {
		args = args[1:]
	}
	switch name {
	case "incident":
		config.IncidentName = strings.Join(args, " ")
		incident.RemoveICS309s()
	case "activation":
		if len(args) > 1 {
			io.WriteString(os.Stderr, "ERROR: activation number must be a single word\n")
			return false
		}
		config.ActivationNum, config.ActNumRequested = strings.Join(args, ""), true
		incident.RemoveICS309s()
	case "period":
		var sd, st, ed, et string
		var sdt, edt time.Time
		var err error

		switch len(args) {
		case 0:
			config.OpStartDate, config.OpStartTime, config.OpEndDate, config.OpEndTime = "", "", "", ""
			break
		case 3:
			sd, st, ed, et = args[0], args[1], args[0], args[2]
		case 4:
			sd, st, ed, et = args[0], args[1], args[2], args[3]
		default:
			io.WriteString(os.Stderr, "ERROR: operational period must be MM/DD/YY HH:MM [MM/DD/YY] HH:MM\n")
			return false
		}
		if sdt, err = time.ParseInLocation("1/2/2006 15:04", sd+" "+st, time.Local); err != nil {
			io.WriteString(os.Stderr, "ERROR: operational period must be MM/DD/YY HH:MM [MM/DD/YY] HH:MM\n")
			return false
		}
		if edt, err = time.ParseInLocation("1/2/2006 15:04", ed+" "+et, time.Local); err != nil {
			io.WriteString(os.Stderr, "ERROR: operational period must be MM/DD/YY HH:MM [MM/DD/YY] HH:MM\n")
			return false
		}
		if !edt.After(sdt) {
			io.WriteString(os.Stderr, "ERROR: end of operational period must be after start\n")
			return false
		}
		config.OpStartDate, config.OpStartTime, config.OpEndDate, config.OpEndTime = sd, st, ed, et
		incident.RemoveICS309s()
	case "operator":
		switch len(args) {
		case 0:
			config.OpCall, config.OpName = "", ""
		case 1:
			io.WriteString(os.Stderr, "ERROR: provide valid FCC call sign followed by operator name\n")
			return false
		default:
			if !fccCallSignRE.MatchString(args[0]) {
				io.WriteString(os.Stderr, "ERROR: provide valid FCC call sign followed by operator name\n")
				return false
			}
			config.OpCall, config.OpName = strings.ToUpper(args[0]), strings.Join(args[1:], " ")
		}
		incident.RemoveICS309s()
	case "tactical":
		switch len(args) {
		case 0:
			config.TacCall, config.TacName = "", ""
		case 1:
			io.WriteString(os.Stderr, "ERROR: provide valid tactical call sign followed by station name\n")
			return false
		default:
			if !tacCallSignRE.MatchString(args[0]) || fccCallSignRE.MatchString(args[0]) {
				io.WriteString(os.Stderr, "ERROR: provide valid tactical call sign followed by station name\n")
				return false
			}
			config.TacCall, config.TacName = strings.ToUpper(args[0]), strings.Join(args[1:], " ")
		}
		config.TacRequested = true
		incident.RemoveICS309s()
	case "bbs":
		var invalid bool

		switch len(args) {
		case 0:
			config.BBS, config.BBSAddress = "", ""
		case 1:
			if ax25RE.MatchString(args[0]) {
				config.BBSAddress = strings.ToUpper(args[0])
				config.BBS, _, _ = strings.Cut(config.BBSAddress, "-")
				break
			}
			_, port, err := net.SplitHostPort(args[0])
			if err != nil {
				invalid = true
				break
			}
			if p, err := strconv.Atoi(port); err != nil || p <= 1024 || p > 65535 {
				invalid = true
				break
			}
			idx := strings.IndexFunc(args[0], func(r rune) bool {
				return (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
			})
			if idx < 0 || !fccCallSignRE.MatchString(args[0][:idx]) {
				invalid = true
				break
			}
			config.BBS, config.BBSAddress = strings.ToUpper(args[0][:idx]), args[0]
		default:
			invalid = true
		}
		if invalid {
			io.WriteString(os.Stderr, "ERROR: bbs must be AX.25 address (W6XSC-1) or telnet host:port\n")
			return false
		}
	case "tnc":
		switch len(args) {
		case 0:
			config.SerialPort = ""
		case 1:
			if runtime.GOOS == "windows" {
				if comPortRE.MatchString(args[0]) {
					config.SerialPort = strings.TrimRight(strings.ToUpper(args[0]), ":")
				} else {
					io.WriteString(os.Stderr, "ERROR: tnc must be COM1 through COM9\n")
					return false
				}
			} else {
				if info, err := os.Stat(args[0]); err == nil || info.Mode().Type()&os.ModeCharDevice != 0 {
					config.SerialPort = args[0]
				} else {
					fmt.Fprintf(os.Stderr, "ERROR: %q is not a character device file\n", args[0])
					return false
				}
			}
		default:
			io.WriteString(os.Stderr, "ERROR: serial port must be a single word\n")
			return false
		}
	case "password":
		// This will break a password containing leading whitespace,
		// trailing whitespace, or whitespace runs other than a single
		// space.  That's probably fine.
		config.Password = strings.Join(args, " ")
	case "msgid":
		switch len(args) {
		case 0:
			config.MessageID = ""
		case 1:
			if match := msgIDRE.FindStringSubmatch(strings.ToUpper(args[0])); match != nil && (match[3] == "M" || match[3] == "P") {
				seq, _ := strconv.Atoi(match[2])
				config.MessageID = fmt.Sprintf("%s-%03d%s", match[1], seq, match[3])
			} else {
				fmt.Fprintf(os.Stderr, "ERROR: %q is not a valid XXX-###P message ID\n", args[0])
				return false
			}
		default:
			io.WriteString(os.Stderr, "ERROR: message ID must be a single word\n")
			return false
		}
	case "defbody":
		config.DefBody = strings.Join(args, " ")
	default:
		fmt.Fprintf(os.Stderr, "ERROR: no such setting %q\n", name)
		return false
	}
	saveConfig()
	return true
}

func helpSet() {
	io.WriteString(os.Stdout, `The "set" command sets incident/activation configuration settings.
    usage: packet set [<name> [=] [<value>]]
With no arguments, it displays the values of all configuration settings.
With only a setting name, it displays the value of that setting.
With a name and value, it sets that setting to that value.
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
`)
}

// requestConfig verifies that all of the requested variables are set, and
// requests them from the user if they aren't.  As a special case, "password" is
// only requested if the setting of "bbs" is a telnet address, and "tnc" is only
// requested if the setting of "bbs" is an AX.25 address.  (For this reason,
// "bbs" should be requested before either of them.)
func requestConfig(vars ...string) bool {
	var scan = bufio.NewScanner(os.Stdin)

	for _, name := range vars {
		var setting *string
		var optional bool

		switch name {
		case "incident":
			setting = &config.IncidentName
		case "activation":
			if !config.ActNumRequested {
				setting, optional, config.ActNumRequested = &config.ActivationNum, true, true
			} else {
				continue
			}
		case "period":
			setting = &config.OpStartDate
		case "operator":
			setting = &config.OpCall
		case "tactical":
			if !config.TacRequested {
				setting, optional, config.TacRequested = &config.TacCall, true, true
			} else {
				continue
			}
		case "bbs":
			setting = &config.BBSAddress
		case "tnc":
			if !ax25RE.MatchString(config.BBSAddress) {
				continue
			}
			setting = &config.SerialPort
		case "password":
			if strings.IndexByte(config.BBSAddress, ':') < 0 {
				continue
			}
			setting = &config.Password
		case "msgid":
			setting = &config.MessageID
		case "defbody":
			setting, optional = &config.DefBody, true
		default:
			panic("unknown variable " + name)
		}
		if *setting != "" {
			continue // it's already set
		}
		for {
			fmt.Printf("set %-10s = ", name)
			if !scan.Scan() {
				return false
			}
			args := strings.Fields(scan.Text())
			if len(args) == 0 && !optional {
				return false
			}
			if setSetting(append([]string{name}, args...)) {
				break
			}
		}
	}
	return true
}
