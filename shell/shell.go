// Package shell contains the main program of the packet shell.  It is called by
// shims that register message types before calling it.
package shell

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rothskeller/packet/incident"
)

// Main is the main program for the packet shell.  Message types should be
// registered before calling it.
func Main() {
	var input *bufio.Scanner

	if !checkDirectory() {
		return
	}
	if len(os.Args) > 1 {
		if ok, _ := runCommand(os.Args[1:], false); !ok {
			os.Exit(1)
		}
		return
	}
	io.WriteString(os.Stdout, "\033[1mPacket Shell v1.6.1 by Steve Roth KC6RSC.  Type \"help\" for help.\n")
	input = bufio.NewScanner(os.Stdin)
	for {
		io.WriteString(os.Stdout, "\033[1mpacket>\033[0m ")
		if !input.Scan() {
			break
		}
		if _, quit := runCommand(strings.Fields(input.Text()), true); quit {
			break
		}
	}
}

// runCommand runs a single command.  The returned ok flag is true if the
// command was recognized and ran successfully.  The returned quit flag is true
// if the command was a quit command.
func runCommand(args []string, inShell bool) (ok, quit bool) {
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "b", "bulletin", "bulletins":
		ok = cmdBulletin(args[1:])
	case "c", "connect", "sr", "rs":
		ok = cmdConnect(args[1:], 1, 1)
	case "ci", "sri", "rsi":
		ok = cmdConnect(args[1:], 2, 2)
	case "delete":
		ok = cmdDelete(args[1:])
	case "draft":
		ok = cmdDraft(args[1:])
	case "e", "edit":
		ok = cmdEdit(args[1:])
	case "h", "help", "--help", "?":
		ok = cmdHelp(args[1:], inShell)
	case "ics309":
		ok = cmdICS309(args[1:])
	case "l", "list":
		ok = cmdList(args[1:])
	case "n", "new":
		ok = cmdNew(args[1:])
	case "pdf":
		ok = cmdShow(args[1:], "pdf")
	case "queue":
		ok = cmdQueue(args[1:])
	case "q", "quit":
		return true, true
	case "r", "receive":
		ok = cmdConnect(args[1:], 0, 1)
	case "reply":
		ok = cmdReply(args[1:])
	case "resend":
		ok = cmdResend(args[1:])
	case "ri":
		ok = cmdConnect(args[1:], 0, 2)
	case "s", "send":
		ok = cmdConnect(args[1:], 1, 0)
	case "set":
		ok = cmdSet(args[1:])
	case "show":
		ok = cmdShow(args[1:], "")
	case "si":
		ok = cmdConnect(args[1:], 2, 0)
	default:
		ok = cmdDefault(args)
	}
	return
}

// cmdDefault implements the default command, i.e., a command line starting with
// a word that isn't a recognized command.
func cmdDefault(args []string) (ok bool) {
	// Find out if the word is a message ID.
	switch lmis := expandMessageID(args[0], true); len(lmis) {
	case 0:
		if args[0] == "309" {
			ok = cmdICS309(args[1:])
		} else {
			io.WriteString(os.Stderr, "ERROR: unknown command or message.  Type \"help\" for help.\n")
		}
	case 1:
		// It is a message ID.  Read the message to determine whether
		// it has been finalized.  We "show" it if it has and "edit" it
		// if it hasn't.
		if env, _, err := incident.ReadMessage(lmis[0]); err != nil {
			ok = false
		} else if env.IsFinal() {
			ok = cmdShow(args, "")
		} else {
			ok = cmdEdit(args)
		}
	default:
		fmt.Fprintf(os.Stderr, "ERROR: %q is ambiguous (%s)\n", args[0], strings.Join(lmis, ", "))
	}
	return ok
}

func cmdHelp(args []string, inShell bool) bool {
	switch len(args) {
	case 0:
		helpTop(inShell)
	case 1:
		switch args[0] {
		case "bulletin", "bulletins", "b":
			helpBulletin()
		case "connect", "c", "ci", "receive", "r", "ri", "send", "s", "si", "rs", "rsi", "sr", "sri":
			helpConnect()
		case "delete":
			helpDelete()
		case "draft":
			helpDraft()
		case "edit", "e":
			helpEdit()
		case "ics309", "309":
			helpICS309()
		case "list", "l":
			helpList()
		case "new", "n":
			helpNew()
		case "queue":
			helpQueue()
		case "reply":
			helpReply()
		case "resend":
			helpResend()
		case "set":
			helpSet()
		case "show", "pdf":
			helpShow()
		case "help", "h", "?", "--help", "quit", "q":
			helpTop(inShell)
		default:
			fmt.Fprintf(os.Stderr, "ERROR: no such command %q\n", args[0])
			helpTop(inShell)
		}
	default:
		fmt.Fprintf(os.Stderr, "usage: packet help [<command>]\n")
		return false
	}
	return true
}

func helpTop(inShell bool) {
	io.WriteString(os.Stdout, `usage: packet [<command>]
Commands are:
    bulletins  schedule and perform bulletin checks
    connect    connect, send queued messages, and receive messages
    delete     delete an unsent message completely
    draft      remove an unsent message from the send queue
    edit       edit an unsent outgoing message
    help       provide help on the packet shell or its commands
    ics309     generate an ICS-309 form with all messages
    list       list messages
    new        create a new outgoing message
    queue      queue an unsent message to be sent
    receive    connect and receive incoming messages (no send)
    reply      create a new reply to a received message
    send       connect and send queued messages (no receive)
    set        set incident/activation and connection parameters
    show       show a message
`)
	if inShell {
		io.WriteString(os.Stdout, "    quit       quit the packet shell\nFor more information about a command, type \"help <command>\".\n")
	} else {
		io.WriteString(os.Stdout, "For more information about a command, run \"packet help <command>\".\n")
	}
}

var msgIDRE = regexp.MustCompile(`^([0-9][A-Z]{2}|[A-Z][A-Z0-9]{2})-([0-9]*[1-9][0-9]*)([A-Z]?)$`)

// expandMessageID searches all messages in the current directory for those
// whose local message ID matches the supplied input.  It returns their full
// local message IDs.  If it doesn't find any, and remoteOK is true, it searches
// remote message IDs as well (and returns the corresponding local message IDs).
func expandMessageID(in string, remoteOK bool) (matches []string) {
	var (
		seq int
		err error
	)
	if incident.MessageExists(in) {
		return []string{in}
	}
	if remoteOK {
		if lmi := incident.LMIForRMI(in); lmi != "" {
			return []string{lmi}
		}
	}
	if seq, err = strconv.Atoi(in); err != nil || seq <= 0 {
		return nil
	}
	if matches, err = incident.SeqToLMI(seq, false); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		return nil
	}
	if len(matches) == 0 && remoteOK {
		matches, _ = incident.SeqToLMI(seq, true)
	}
	return matches
}

// checkDirectory returns whether our current working directory is a valid
// incident directory.  If not, it prints a message to stderr and returns false.
func checkDirectory() bool {
	var (
		home string
		cwd  string
		err  error
	)
	if cwd, err = os.Getwd(); err != nil {
		// Don't know where we are, so assume it's not a prohibited
		// place.
		goto CHECK_WRITE
	}
	if cwd, err = filepath.EvalSymlinks(cwd); err != nil {
		goto CHECK_WRITE
	}
	if home = os.Getenv("HOME"); home == "" {
		// Don't know where home is, so assume current dir is not
		// prohibited.
		goto CHECK_WRITE
	}
	if home, err = filepath.EvalSymlinks(home); err != nil {
		goto CHECK_WRITE
	}
	// Check for a prohibited location.
	if home == cwd || cwd == filepath.Join(home, "Desktop") ||
		cwd == filepath.Join(home, "Documents") || cwd == "/" || cwd == "C:\\" {
		fmt.Fprintf(os.Stderr, `ERROR: current working directory is not suitable
Please create a new directory for this incident/activation and cd into it
before running "packet".  Or, to resume an existing incident/activation,
cd into its directory before running "packet".
`)
		return false
	}
CHECK_WRITE:
	// Make sure we can create files in it.
	if fh, err := os.Create(".packet.testcreate"); err == nil {
		fh.Close()
		os.Remove(".packet.testcreate")
	} else {
		fmt.Fprintf(os.Stderr, `ERROR: current working directory is not writable
Please cd into a writable directory before running "packet".  Either cd
into the directory for an existing incident/activation, or create a new
directory and cd into it for a new incident/activation.
`)
		return false
	}
	return true
}
