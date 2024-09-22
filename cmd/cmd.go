package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/spf13/pflag"
)

var safeDir string

func Run(args []string) (ok bool) {
	var err error

	defer func() {
		if p := recover(); p != nil {
			if logfile, err := os.OpenFile("packet.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666); err == nil {
				fmt.Fprintf(logfile, "PANIC: %v\n%s\n", p, debug.Stack())
				logfile.Close()
			}
			panic(p)
		}
	}()
	cio.Detect()
	safeDir = safeDirectory()
	if len(args) == 0 {
		err = shell()
	} else if safeDir != "" {
		err = errors.New(safeDir)
	} else {
		err = run(args)
	}
	if err != nil && err != ErrQuit {
		cio.Error(err.Error())
		return false
	}
	return true
}

func run(args []string) (err error) {
	switch args[0] {
	case "b", "bull", "bulletin", "bulletins":
		return cmdBulletins(args[1:])
	case "cd", "chdir", "md", "mkdir", "pwd":
		return cmdChdir(args[0], args[1:])
	case "c", "connect":
		return cmdConnect(args[1:])
	case "delete":
		return cmdDelete(args[1:])
	case "draft":
		return cmdDraft(args[1:])
	case "dump":
		return cmdDump(args[1:])
	case "e", "edit":
		return cmdEdit(args[1:])
	case "h", "help", "--help", "-?":
		return cmdHelp(args[1:])
	case "309", "ics309":
		return cmdICS309(args[1:])
	case "l", "list":
		return cmdList(args[1:])
	case "n", "new":
		return cmdNew(args[1:])
	case "pdf":
		return cmdPDF(args[1:])
	case "queue":
		return cmdQueue(args[1:])
	case "q", "quit", "exit":
		return ErrQuit
	case "set":
		return cmdSet(args[1:])
	case "s", "show":
		return cmdShow(args[1:])
	case "version":
		return cmdVersion(args[1:])
	case "config", "script", "types":
		return cmdHelp(args[1:])
	default:
		return fmt.Errorf("no such command %q", args[0])
	}
}

func shell() (err error) {
	if !cio.InputIsTerm || !cio.OutputIsTerm {
		return ErrUsage("usage: packet «command»\n       packet help\n")
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
		cio.Welcome(`Packet Shell %s de KC6RSC.  Type "help" for help.`, info.Main.Version)
	} else {
		cio.Welcome(`Packet Shell de KC6RSC.  Type "help" for help.`)
	}
	if safeDir != "" {
		cio.Confirm(`WARNING: %s  Use the "cd" command to switch to a different directory.`, safeDir)
	}
	for {
		var (
			line    string
			args    []string
			in      *os.File
			out     *os.File
			saveIn  *os.File
			saveOut *os.File
		)
		// If the directory is not safe, give a warning to them to
		// change it.  Then clear the problem so we don't trip over it
		// again.
		if safeDir != "" {
			cio.Confirm(`WARNING: %s  Use the "cd" command to switch to a different directory.`, safeDir)
			safeDir = ""
		}
		// Read and parse the command line.
		if line, err = cio.ReadCommand(); err != nil {
			if err == io.EOF {
				line = "quit"
			} else {
				return err
			}
		}
		if args, in, out, err = parseCommandLine(line); err != nil {
			cio.Error(err.Error())
			continue
		}
		if len(args) != 0 && args[0] == "packet" {
			// Just in case they type "packet foo" while already in
			// the shell.
			args = args[1:]
		}
		if len(args) == 0 {
			continue
		}
		// Save the old stdin and stdout and apply the new ones.
		saveIn, saveOut = os.Stdin, os.Stdout
		if in != nil {
			os.Stdin = in
		}
		if out != nil {
			os.Stdout = out
		}
		cio.Detect()
		// Run the command.
		err = run(args)
		// Restore the old stdin and stdout.
		os.Stdin, os.Stdout = saveIn, saveOut
		if in != nil {
			in.Close()
		}
		if out != nil {
			out.Close()
		}
		cio.Detect()
		// Handle the result of the command.
		if err != nil && err != ErrQuit {
			cio.Error(err.Error())
		}
		if err == ErrQuit {
			return nil
		}
	}
}

// expandMessageID searches all messages in the current directory for those
// whose local message ID matches the supplied input.  If it finds exactly one,
// it returns the full message ID.  If it finds more than one, it returns an
// error.  If it doesn't find any, and remoteOK is true, it searches remote
// message IDs as well.  Again, if it finds exactly one match, it returns the
// full local message ID of that message.  If it finds more than one, or none,
// it returns an error.
func expandMessageID(in string, remoteOK bool) (lmi string, err error) {
	var (
		seq     int
		matches []string
	)
	inUC := strings.ToUpper(in)
	if incident.MessageExists(inUC) {
		return inUC, nil
	}
	if remoteOK {
		if lmi := incident.LMIForRMI(inUC); lmi != "" {
			return lmi, nil
		}
	}
	if seq, err = strconv.Atoi(in); err != nil || seq <= 0 {
		return "", fmt.Errorf("%q is not a valid message ID or number", in)
	}
	if matches, err = incident.SeqToLMI(seq, false); err != nil {
		return "", err
	}
	if len(matches) == 0 && remoteOK {
		matches, _ = incident.SeqToLMI(seq, true)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no such message %q", in)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%q is ambiguous: it could be %s", in, strings.Join(matches, ", "))
	}
}

// expandFieldName finds the message field that (best) matches the supplied
// field name.  If loose is true, it can be a partial, heuristic match.
func expandFieldName(fields []*message.Field, in string, loose bool) (*message.Field, error) {
	// First priority is a match on PIFO tag.
	for _, f := range fields {
		if f.PIFOTag == in {
			return f, nil
		}
	}
	// Remaining comparisons are case-sensitive if the input contains any
	// uppercase letters.
	var caseSensitive bool
	var compare func(string, string) bool
	if strings.IndexFunc(in, func(r rune) bool { return r >= 'A' && r <= 'Z' }) >= 0 {
		compare, caseSensitive = func(a, b string) bool { return a == b }, true
	} else {
		compare, caseSensitive = strings.EqualFold, false
	}
	// Second priority is a case-insensitive match on full field name.
	for _, f := range fields {
		if compare(f.Label, in) {
			return f, nil
		}
	}
	// To avoid the need for quoting, we will also accept a case-insensitive
	// match on the full field name with spaces removed.
	if !strings.Contains(in, " ") {
		for _, f := range fields {
			if compare(strings.ReplaceAll(f.Label, " ", ""), in) {
				return f, nil
			}
		}
	}
	// Unless the loose flag is set, those are the only options.
	if !loose {
		return nil, fmt.Errorf("no such field %q (loose matches are not allowed in batch mode)", in)
	}
	// Now we look for fields whose name contains the same characters as the
	// input, in the same order, but also contains additional characters.
	// We return the field that has the smallest number of unmatched
	// capital letters in its name, and among those, the one with the
	// smallest number of unmatched characters.
	var match *message.Field
	var missedUC, missedCH int
	for _, f := range fields {
		if mUC, mCH, ok := matchFieldName(f.Label, in, caseSensitive); ok {
			if match == nil || mUC < missedUC || (mUC == missedUC && mCH < missedCH) {
				match, missedUC, missedCH = f, mUC, mCH
			}
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no such field %q", in)
	}
	return match, nil
}

// matchFieldName is a recursive function that determines whether the input is a
// valid shortening of the field name, and returns the heuristic scoring if so.
// It's a pretty expensive algorithm, but at human time scales it's negligible.
func matchFieldName(fname, in string, caseSensitive bool) (mUC, mCH int, ok bool) {
	if in == "" && fname == "" {
		// Nothing left of either string.  Perfect match.
		return 0, 0, true
	}
	if in == "" {
		// Nothing left of in, but we still have some fname.  Compute
		// the score.  Use a recursive call to get the score after
		// removing the first character of fname, then add the score
		// for that character.
		mUC, mCH, _ = matchFieldName(fname[1:], in, caseSensitive)
		if fname[0] >= 'A' && fname[0] <= 'Z' {
			mUC++
		}
		mCH++
		return mUC, mCH, true
	}
	if fname == "" {
		// Nothing left of fname, but we still have some in.  Not a
		// match at all.
		return 0, 0, false
	}
	var mUC1, mCH1, mUC2, mCH2 int
	var ok1, ok2 bool
	// If the lead characters of fname and in match, calculate the score
	// based on matching those two.
	if fname[0] == in[0] || (!caseSensitive && downcase(fname[0]) == downcase(in[0])) {
		mUC1, mCH1, ok1 = matchFieldName(fname[1:], in[1:], caseSensitive)
	}
	// Whether the lead characters match or not, also calculate the score
	// assuming they don't.
	mUC2, mCH2, ok2 = matchFieldName(fname[1:], in, caseSensitive)
	if fname[0] >= 'A' && fname[0] <= 'Z' {
		mUC2++
	}
	mCH2++
	// Return the better of the two scores.
	if !ok1 && !ok2 {
		return 0, 0, false
	}
	if !ok1 {
		return mUC2, mCH2, ok2
	}
	if !ok2 || mUC1 < mUC2 || (mUC1 == mUC2 && mCH1 < mCH2) {
		return mUC1, mCH1, ok1
	}
	return mUC2, mCH2, ok2
}
func downcase(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b + 'A' - 'a'
	}
	return b
}

// parseCommandLine parses a command line that the user typed at our command
// line.  It interprets redirection.
func parseCommandLine(line string) (args []string, in, out *os.File, err error) {
	args = tokenizeLine(line)
	i := 0
	for i < len(args) {
		if args[i] == "<" {
			if in != nil {
				return nil, nil, nil, errors.New("multiple input redirects on command line")
			}
			if i == len(args)-1 || args[i+1] == "<" || args[i+1] == ">" || args[i+1] == ">>" {
				return nil, nil, nil, errors.New("missing filename after <")
			}
			if in, err = os.Open(args[i+1]); err != nil {
				return nil, nil, nil, err
			}
			args = append(args[:i], args[i+2:]...)
			continue
		}
		if args[i] == ">" || args[i] == ">>" {
			if out != nil {
				return nil, nil, nil, errors.New("multiple output redirects on command line")
			}
			if i == len(args)-1 || args[i+1] == "<" || args[i+1] == ">" || args[i+1] == ">>" {
				return nil, nil, nil, errors.New("missing filename after " + args[i])
			}
			if args[i] == ">" {
				out, err = os.Create(args[i+1])
			} else {
				out, err = os.OpenFile(args[i+1], os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
			}
			if err != nil {
				return nil, nil, nil, err
			}
			args = append(args[:i], args[i+2:]...)
			continue
		}
		i++
	}
	return args, in, out, nil
}

// tokenizeLine parses a received command line into words.  It supports
// rudimentary quoting with either ' or ".  It does not support backslash
// escapes.  It treats unquoted <, >, and >> as separate words even when not
// surrounded by whitespace.
func tokenizeLine(line string) (args []string) {
	var partial string
	var quoted bool

	for line != "" {
		idx := strings.IndexAny(line, " \t\f\r\n'\"<>")
		if idx < 0 {
			partial += line
			args = append(args, partial)
			return args
		}
		if line[idx] == '\'' || line[idx] == '"' {
			partial += line[:idx]
			idx2 := strings.IndexByte(line[idx+1:], line[idx])
			if idx2 < 0 {
				partial += line[idx+1:]
				args = append(args, partial)
				return args
			}
			idx2 += idx + 1 // make it an offset into line
			partial += line[idx+1 : idx2]
			quoted = true
			line = line[idx2+1:]
			continue
		}
		if line[idx] == '>' && idx < len(line)-1 && line[idx+1] == '>' {
			if partial != "" || quoted {
				args, partial, quoted = append(args, partial), "", false
			}
			args, line = append(args, line[idx:idx+2]), line[idx+2:]
			continue
		}
		if line[idx] == '<' || line[idx] == '>' {
			if partial != "" || quoted {
				args, partial, quoted = append(args, partial), "", false
			}
			args, line = append(args, line[idx:idx+1]), line[idx+1:]
			continue
		}
		partial += line[:idx]
		if partial != "" || quoted {
			args, partial, quoted = append(args, partial), "", false
		}
		line = line[idx+1:]
	}
	if partial != "" || quoted {
		args = append(args, partial)
	}
	return args
}

func safeDirectory() string {
	var (
		cwd  string
		home string
		temp *os.File
		err  error
	)
	if cwd, err = os.Getwd(); err != nil {
		return `The current directory is not readable.`
	}
	if np, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = np
	}
	if home, err = os.UserHomeDir(); err == nil && home != "" {
		if cwd == home {
			return `The current directory is the user's home directory.  This is not a suitable location for message storage because each incident should have its own directory.`
		}
		if cwd == filepath.Join(home, "Desktop") {
			return `The current directory is the user's desktop.  This is not a suitable location for message storage because each incident should have its own directory.`
		}
		if cwd == filepath.Join(home, "Documents") {
			return `The current directory is the user's documents folder.  This is not a suitable location for message storage because each incident should have its own directory.`
		}
	}
	if cwd == "/" || (len(cwd) == 3 && cwd[1] == ':' && cwd[2] == '\\') {
		return `The current directory is the file system root.  This is not a suitable location for message storage because each incident should have its own directory.`
	}
	if temp, err = os.Create(".test-for-writable"); err != nil {
		return `The current directory is not writable.  No messages can be stored here.`
	}
	temp.Close()
	os.Remove(".test-for-writable")
	return ""
}

func gaveMutuallyExclusiveFlags(set *pflag.FlagSet, flags ...string) (err error) {
	var seen bool

	for _, flag := range flags {
		if f := set.Lookup(flag); f.Changed {
			if seen {
				if len(flags) == 2 {
					return fmt.Errorf("the --%s and --%s flags are incompatible", flags[0], flags[1])
				} else {
					return fmt.Errorf("the --%s and --%s flags are incompatible", strings.Join(flags[:len(flags)-1], ", --"), flags[len(flags)-1])
				}
			}
			seen = true
		}
	}
	return nil
}
