package config

import (
	"bufio"
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

	"github.com/rothskeller/packet/incident"
)

var (
	fccCallSignRE = regexp.MustCompile(`(?i)^(?:A[A-L][0-9][A-Z]{1,3}|[KNW][0-9][A-Z]{2,3}|[KNW][A-Z][0-9][A-Z]{1,3})$`)
	tacCallSignRE = regexp.MustCompile(`(?i)^[A-Z][A-Z0-9]{5}$`)
	ax25RE        = regexp.MustCompile(`(?i)^(?:A[A-L][0-9][A-Z]{1,3}|[KNW][0-9][A-Z]{2,3}|[KNW][A-Z][0-9][A-Z]{1,3})-(?:1[0-5]|[1-9])$`)
	comPortRE     = regexp.MustCompile(`(?i)COM[1-9]:?$`)
	MsgIDRE       = regexp.MustCompile(`^([0-9][A-Z]{2}|[A-Z][A-Z0-9]{2})-([0-9]*[1-9][0-9]*)([A-Z]?)$`)
)

// packetConf is the name of the packet configuration file.
const packetConf = "packet.conf"

// packetDefaults is the name of the defaults file, stored in the user's HOME.
const packetDefaults = ".packet"

type PacketConfig struct {
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
	Bulletins       map[string]*BulletinConfig `json:",omitempty"`
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
	if err = json.NewDecoder(fh).Decode(&C); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s: %s", filename, err)
		return
	}
}

func SetSetting(name, value string) error {
	switch name {
	case "incident":
		C.IncidentName = value
		incident.RemoveICS309s()
	case "activation":
		C.ActivationNum, C.ActNumRequested = value, true
		incident.RemoveICS309s()
	case "period":
		var words = strings.Fields(value)
		var sd, st, ed, et string
		var sdt, edt time.Time
		var err error

		switch len(words) {
		case 0:
			C.OpStartDate, C.OpStartTime, C.OpEndDate, C.OpEndTime = "", "", "", ""
		case 3:
			sd, st, ed, et = words[0], words[1], words[0], words[2]
		case 4:
			sd, st, ed, et = words[0], words[1], words[2], words[3]
		default:
			return errors.New("operational period must be MM/DD/YY HH:MM [MM/DD/YY] HH:MM")
		}
		if sdt, err = time.ParseInLocation("1/2/2006 15:04", sd+" "+st, time.Local); err != nil {
			return errors.New("operational period must be MM/DD/YY HH:MM [MM/DD/YY] HH:MM")
		}
		if edt, err = time.ParseInLocation("1/2/2006 15:04", ed+" "+et, time.Local); err != nil {
			return errors.New("operational period must be MM/DD/YY HH:MM [MM/DD/YY] HH:MM")
		}
		if !edt.After(sdt) {
			return errors.New("end of operational period must be after start")
		}
		C.OpStartDate, C.OpStartTime, C.OpEndDate, C.OpEndTime = sd, st, ed, et
		incident.RemoveICS309s()
	case "operator":
		var words = strings.Fields(value)
		switch len(words) {
		case 0:
			C.OpCall, C.OpName = "", ""
		case 1:
			return errors.New("provide valid FCC call sign followed by operator name")
		default:
			if !fccCallSignRE.MatchString(words[0]) {
				return errors.New("provide valid FCC call sign followed by operator name")
			}
			C.OpCall, C.OpName = strings.ToUpper(words[0]), strings.Join(words[1:], " ")
		}
		incident.RemoveICS309s()
	case "tactical":
		var words = strings.Fields(value)
		switch len(words) {
		case 0:
			C.TacCall, C.TacName = "", ""
		case 1:
			return errors.New("provide valid tactical call sign followed by station name")
		default:
			if !tacCallSignRE.MatchString(words[0]) || fccCallSignRE.MatchString(words[0]) {
				return errors.New("provide valid tactical call sign followed by station name")
			}
			C.TacCall, C.TacName = strings.ToUpper(words[0]), strings.Join(words[1:], " ")
		}
		C.TacRequested = true
		incident.RemoveICS309s()
	case "bbs":
		if value == "" {
			C.BBS, C.BBSAddress = "", ""
		} else if ax25RE.MatchString(value) {
			C.BBSAddress = strings.ToUpper(value)
			C.BBS, _, _ = strings.Cut(C.BBSAddress, "-")
		} else {
			_, port, err := net.SplitHostPort(value)
			if err != nil {
				return errors.New("bbs must be AX.25 address (W6XSC-1) or telnet host:port")
			}
			if p, err := strconv.Atoi(port); err != nil || p <= 1024 || p > 65535 {
				return errors.New("bbs must be AX.25 address (W6XSC-1) or telnet host:port")
			}
			idx := strings.IndexFunc(value, func(r rune) bool {
				return (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
			})
			if idx < 0 || !fccCallSignRE.MatchString(value[:idx]) {
				return errors.New("bbs must be AX.25 address (W6XSC-1) or telnet host:port")
			}
			C.BBS, C.BBSAddress = strings.ToUpper(value[:idx]), value
		}
	case "tnc":
		if value == "" {
			C.SerialPort = ""
		} else {
			if runtime.GOOS == "windows" {
				if comPortRE.MatchString(value) {
					C.SerialPort = strings.TrimRight(strings.ToUpper(value), ":")
				} else {
					return errors.New("tnc must be COM1 through COM9")
				}
			} else {
				if info, err := os.Stat(value); err == nil || info.Mode().Type()&os.ModeCharDevice != 0 {
					C.SerialPort = value
				} else {
					return fmt.Errorf("%q is not a character device file", value)
				}
			}
		}
	case "password":
		C.Password = value
	case "msgid":
		if value == "" {
			C.MessageID = ""
		} else {
			if match := MsgIDRE.FindStringSubmatch(strings.ToUpper(value)); match != nil && (match[3] == "M" || match[3] == "P") {
				seq, _ := strconv.Atoi(match[2])
				C.MessageID = fmt.Sprintf("%s-%03d%s", match[1], seq, match[3])
			} else {
				return fmt.Errorf("%q is not a valid XXX-###P message ID", value)
			}
		}
	case "defbody":
		C.DefBody = value
	default:
		return fmt.Errorf("no such setting %q", name)
	}
	SaveConfig()
	return nil
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
	reduced.Bulletins = make(map[string]*BulletinConfig, len(C.Bulletins))
	for area, bc := range C.Bulletins {
		reduced.Bulletins[area] = &BulletinConfig{Frequency: bc.Frequency}
	}
	by, _ = json.Marshal(&reduced)
	if err = os.WriteFile(filepath.Join(home, packetDefaults), by, 0666); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s", err)
	}
}

// RequestConfig verifies that all of the requested variables are set, and
// requests them from the user if they aren't.  As a special case, "password" is
// only requested if the setting of "bbs" is a telnet address, and "tnc" is only
// requested if the setting of "bbs" is an AX.25 address.  (For this reason,
// "bbs" should be requested before either of them.)
func RequestConfig(vars ...string) (err error) {
	var scan = bufio.NewScanner(os.Stdin)

	for _, name := range vars {
		var setting *string
		var optional bool

		switch name {
		case "incident":
			setting = &C.IncidentName
		case "activation":
			if !C.ActNumRequested {
				setting, optional, C.ActNumRequested = &C.ActivationNum, true, true
			} else {
				continue
			}
		case "period":
			setting = &C.OpStartDate
		case "operator":
			setting = &C.OpCall
		case "tactical":
			if !C.TacRequested {
				setting, optional, C.TacRequested = &C.TacCall, true, true
			} else {
				continue
			}
		case "bbs":
			setting = &C.BBSAddress
		case "tnc":
			if !ax25RE.MatchString(C.BBSAddress) {
				continue
			}
			setting = &C.SerialPort
		case "password":
			if strings.IndexByte(C.BBSAddress, ':') < 0 {
				continue
			}
			setting = &C.Password
		case "msgid":
			setting = &C.MessageID
		case "defbody":
			setting, optional = &C.DefBody, true
		default:
			panic("unknown variable " + name)
		}
		if *setting != "" {
			continue // it's already set
		}
		for {
			fmt.Printf("configure %-10s = ", name)
			if !scan.Scan() {
				return scan.Err()
			}
			value := strings.TrimSpace(scan.Text())
			if value == "" && !optional {
				return errors.New("cannot continue without required configuration settings")
			}
			if err = SetSetting(name, value); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			} else {
				break
			}
		}
	}
	return nil
}
