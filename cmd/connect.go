package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet-shell/config"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/jnos"
	"github.com/rothskeller/packet/jnos/kpc3plus"
	"github.com/rothskeller/packet/jnos/telnet"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/delivrcpt"
	"github.com/rothskeller/packet/xscmsg/readrcpt"

	"github.com/spf13/pflag"
)

const connectSlug = `Connect to the BBS to send and/or receive messages`
const connectHelp = `
usage: packet connect [flags]
  -i, --immediate  ⇥immediate messages only
  -r, --receive    ⇥receiving incoming messages
  -s, --send       ⇥send queued messages
  -v, --verbose    ⇥show BBS conversation

The "connect" (or "c") command makes a connection to the BBS and sends and/or receives messages.  With the --send flag, it sends queued outgoing messages; with the --receive flag, it receives incoming messages; with both or neither, it does both.  With the --immediate flag, only immediate messages are sent and/or received.

When receiving messages without the --immediate flag, any scheduled bulletin checks are performed as well.  (See the "packet bulletins" command for scheduling of bulletin checks.)

The "connect" command lists all messages sent and received, except for receipts.  Run "packet help list" for details of the output format.
`

type connection struct {
	tosend        []string
	rcvlevel      int
	subjectToLMI  map[string]string
	areas         map[string]*config.BulletinConfig
	haveBulletins map[string]map[string]bool
	sigintch      chan os.Signal
	conn          *jnos.Conn
}

var ErrInterrupted = errors.New("connection interrupted by Ctrl-C")

func cmdConnect(args []string) (err error) {
	var (
		send      bool
		receive   bool
		immediate bool
		verbose   bool
		sendlevel int
		conn      connection
		flags     = pflag.NewFlagSet("connect", pflag.ContinueOnError)
	)
	flags.BoolVarP(&send, "send", "s", false, "send queued messages")
	flags.BoolVarP(&receive, "receive", "r", false, "receive incoming messages")
	flags.BoolVarP(&immediate, "immediate", "i", false, "immediate messages only")
	flags.BoolVarP(&verbose, "verbose", "v", false, "show BBS conversation")
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"connect"})
	} else if err != nil {
		cio.Error(err.Error())
		return usage(connectHelp)
	}
	if flags.NArg() != 0 {
		return usage(connectHelp)
	}
	if send {
		sendlevel = 1
	}
	if receive {
		conn.rcvlevel = 1
	}
	if sendlevel == 0 && conn.rcvlevel == 0 {
		sendlevel, conn.rcvlevel = 1, 1
	}
	if immediate {
		sendlevel, conn.rcvlevel = sendlevel*2, conn.rcvlevel*2
	}
	// If we're checking bulletins, make a map of the areas to check based
	// on time elapsed and requested frequency.
	if conn.rcvlevel == 1 {
		conn.areas = make(map[string]*config.BulletinConfig)
		for area, bc := range config.C.Bulletins {
			if time.Since(bc.LastCheck) >= bc.Frequency {
				conn.areas[area] = bc
			}
		}
	}
	// Scan through all existing messages, gathering data that we will need
	// to handle the connection
	conn.tosend, conn.subjectToLMI, conn.haveBulletins = preConnectScan(sendlevel, conn.areas)
	// Do we have anything to do?
	if len(conn.tosend) == 0 && conn.rcvlevel == 0 {
		return errors.New("nothing to send")
	}
	if !haveConnectConfig() && cio.InputIsTerm && cio.OutputIsTerm {
		cio.Confirm("Please provide necessary configuration settings for connection:")
		if err = run([]string{"edit", "config"}); err != nil {
			return err
		}
	}
	if !haveConnectConfig() {
		return errors.New("missing necessary configuration settings")
	}
	// Run the connection.
	defer cio.Status("")
	if err := conn.run(verbose); err != nil {
		return err
	}
	// Save the configuration.  It may have new unread messages or new
	// LastCheck times for the bulletin areas.
	config.SaveConfig()
	return nil
}

// preConnectScan scans all existing messages gathering information needed prior
// to a connection.  It returns the list of messages to send, a map from subject
// line to LMI for already-received direct messages, and a map from bulletin
// area to the set of bulletin subjects already retrieved from that area.
func preConnectScan(sendlevel int, areas map[string]*config.BulletinConfig) (
	tosend []string, subjectToLMI map[string]string, haveBulletins map[string]map[string]bool,
) {
	var lmis []string

	subjectToLMI = make(map[string]string)
	haveBulletins = make(map[string]map[string]bool)
	lmis, _ = incident.AllLMIs()
	for _, lmi := range lmis {
		env, _, err := incident.ReadMessage(lmi)
		if err != nil {
			continue
		}
		if env.IsReceived() && env.ReceivedArea != "" && areas != nil && areas[env.ReceivedArea] != nil {
			// It's a bulletin in an area we'll be checking.  Record
			// the subject line so we know not to retrieve it again.
			// Subject lines are truncated to 35 characters and then
			// trimmed by the JNOS list command, so that's what
			// we'll record.
			subject := env.SubjectLine
			if len(subject) > 35 {
				subject = subject[:35]
			}
			subject = strings.TrimSpace(subject)
			if haveBulletins[env.ReceivedArea] == nil {
				haveBulletins[env.ReceivedArea] = make(map[string]bool)
			}
			haveBulletins[env.ReceivedArea][subject] = true
		}
		if !env.IsReceived() && env.IsFinal() {
			// It's a sent message.  Add it to the map from subject
			// line to LMI, in case we see a receipt for it.
			subjectToLMI[env.SubjectLine] = lmi
		}
		if !env.IsFinal() && env.ReadyToSend {
			// It's a message queued to be sent.  Add it to the list
			// to be sent (but limit it to immediate messages only
			// if that was requested).
			if sendlevel == 2 {
				if _, _, handling, _, _ := message.DecodeSubject(env.SubjectLine); handling != "I" {
					return
				}
			}
			if sendlevel != 0 {
				tosend = append(tosend, lmi)
			}
		}
	}
	return
}

// haveConnectConfig returns whether we have all of the necessary config
// settings to make a connection to the server.
func haveConnectConfig() bool {
	if config.C.BBSAddress == "" || config.C.OpCall == "" || config.C.MessageID == "" {
		return false
	}
	if strings.Contains(config.C.BBSAddress, ":") {
		if config.C.Password == "" {
			return false
		}
	} else {
		if config.C.SerialPort == "" {
			return false
		}
	}
	return true
}

// run connects to the BBS and performs the desired operations.  It sends all of
// the messages whose filenames are in the tosend array.  If rcvlevel is 2, it
// receives immediate messages.  If rcvlevel is 1, it receives all incoming
// messages.  If rcvlevel is 0, it does not receive any messages.  All bulletin
// areas listed in areas are checked for new bulletins.
func (c *connection) run(verbose bool) (err error) {
	var (
		mailbox string
		logfile *os.File
		log     io.Writer
	)
	// Intercept ^C so we can close the connection gracefully.
	c.sigintch = make(chan os.Signal, 10)
	signal.Notify(c.sigintch, os.Interrupt)
	defer c.drainSigInt()
	// Append to the log file.
	if logfile, err = os.OpenFile("packet.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666); err != nil {
		return err
	}
	defer logfile.Close()
	if verbose {
		log = io.MultiWriter(logfile, os.Stdout)
		cio.SuppressStatus = true
		defer func() { cio.SuppressStatus = false }()
	} else {
		log = logfile
	}
	// Connect to the BBS.
	if config.C.TacCall != "" {
		mailbox = config.C.TacCall
	} else {
		mailbox = config.C.OpCall
	}
	cio.Status("Connecting to %s@%s...", mailbox, config.C.BBS)
	if strings.IndexByte(config.C.BBSAddress, ':') >= 0 { // internet connection
		c.conn, err = telnet.Connect(config.C.BBSAddress, mailbox, config.C.Password, log)
	} else { // radio connection
		c.conn, err = kpc3plus.Connect(config.C.SerialPort, config.C.BBSAddress, mailbox, config.C.OpCall, log)
	}
	if err != nil {
		return fmt.Errorf("JNOS connect: %s", err)
	}
	defer func() {
		cio.Status("Closing connection...")
		if err2 := c.conn.Close(); err == nil && err2 != nil {
			err = fmt.Errorf("JNOS close: %s", err2)
		}
	}()
	if err = c.sendMessages(); err != nil {
		return fmt.Errorf("send messages: %s", err)
	}
	switch c.rcvlevel {
	case 1:
		if err = c.receiveMessages(); err != nil {
			return fmt.Errorf("receive messages: %s", err)
		}
	case 2:
		if err = c.receiveImmediates(); err != nil {
			return fmt.Errorf("receive immediate messages: %s", err)
		}
	}
	if err = c.receiveBulletins(); err != nil {
		return fmt.Errorf("receive bulletins: %s", err)
	}
	cio.EndMessageList("No messages sent or received.")
	return nil
}

// sendMessages sends the listed messages.
func (c *connection) sendMessages() (err error) {
	for _, lmi := range c.tosend {
		env, msg, err := incident.ReadMessage(lmi)
		if err != nil {
			return fmt.Errorf("send %s: read message: %s", lmi, err)
		}
		if err = c.sendMessage(lmi, env, msg); err != nil {
			return fmt.Errorf("send %s: %s", lmi, err)
		}
	}
	return nil
}

// sendMessage sends a single message.  It is used for both outgoing human
// messages and delivery receipts.
func (c *connection) sendMessage(filename string, env *envelope.Envelope, msg message.Message) (err error) {
	if c.checkSigInt() {
		return ErrInterrupted
	}
	if config.C.TacCall != "" {
		env.From = (&envelope.Address{Name: config.C.TacName, Address: strings.ToLower(config.C.TacCall + "@" + config.C.BBS + ".ampr.org")}).String()
	} else {
		env.From = (&envelope.Address{Name: config.C.OpName, Address: strings.ToLower(config.C.OpCall + "@" + config.C.BBS + ".ampr.org")}).String()
	}
	env.Date = time.Now()
	msg.SetOperator(config.C.OpCall, config.C.OpName, false)
	body := msg.EncodeBody()
	if strings.HasSuffix(filename, ".DR") {
		cio.Status("Sending delivery receipt for %s...", filename[:len(filename)-3])
	} else {
		cio.Status("Sending %s...", filename)
	}
	var to []string
	if addrs, err := envelope.ParseAddressList(env.To); err != nil {
		return errors.New("invalid To: address list")
	} else if len(addrs) == 0 {
		return errors.New("no To: addresses")
	} else {
		to = make([]string, len(addrs))
		for i, a := range addrs {
			to[i] = a.Address
		}
	}
	if err = c.conn.Send(env.SubjectLine, env.RenderBody(body), to...); err != nil {
		return fmt.Errorf("JNOS send: %s", err)
	}
	if strings.HasSuffix(filename, ".DR") {
		if err = incident.SaveReceipt(filename[:len(filename)-3], env, msg); err != nil {
			return fmt.Errorf("save receipt %s: %s", filename, err)
		}
	} else {
		if err = incident.SaveMessage(filename, "", env, msg, false); err != nil {
			return fmt.Errorf("save message %s: %s", filename, err)
		}
	}
	if !strings.HasSuffix(filename, ".DR") {
		cio.ListMessage(listItemForMessage(filename, "", env))
	}
	return nil
}

// receiveMessages receives all messages in the current mailbox (i.e., the one
// we connected to).
func (c *connection) receiveMessages() (err error) {
	var msgnum = 1
	for {
		var done bool
		if done, err = c.receiveMessage("", msgnum); err != nil {
			return fmt.Errorf("receive message %d: %s", msgnum, err)
		}
		if done {
			break
		}
		msgnum++
	}
	return nil
}

// receiveMessage receives a single message with the specified number from the
// current area, and kills it from the server.  It also sends delivery receipts
// when appropriate.  It returns false, nil if everything's OK; true, nil if no
// message with the specified number exists, and false, !nil if some other error
// occurs.
func (c *connection) receiveMessage(area string, msgnum int) (done bool, err error) {
	var rmi string

	if c.checkSigInt() {
		return false, ErrInterrupted
	}
	// Read the message.
	if area != "" {
		cio.Status("Reading message %d in %s...", msgnum, area)
	} else {
		cio.Status("Reading message %d...", msgnum)
	}
	raw, err := c.conn.Read(msgnum)
	if err != nil {
		return false, fmt.Errorf("JNOS read %d: %s", msgnum, err)
	}
	if raw == "" {
		return true, nil
	}
	// Record receipt of the message.
	lmi, env, msg, oenv, omsg, err := incident.ReceiveMessage(
		raw, config.C.BBS, area, config.C.MessageID, config.C.OpCall, config.C.OpName)
	if err == incident.ErrDuplicateReceipt {
		cio.Confirm("NOTE: discarding duplicate receipt for %s", lmi)
		goto KILL
	}
	if err != nil {
		return false, err
	}
	// Received receipts are handled differently than other messages.
	switch msg := msg.(type) {
	case nil:
		// ignore message (e.g. autoresponse)
	case *readrcpt.ReadReceipt:
		// ignore read receipts
	case *delivrcpt.DeliveryReceipt:
		if lmi == "" {
			cio.Confirm("NOTE: discarding receipt for unknown message %q", msg.MessageSubject)
		} else {
			// Display the fact that our message was delivered.
			var li = listItemForMessage(lmi, msg.LocalMessageID, oenv)
			li.Flag = "HAVE RCPT"
			cio.ListMessage(li)
		}
	default:
		// Mark it unread.
		config.C.Unread[lmi] = true
		// Display the newly received message.
		if mb := msg.Base(); mb.FOriginMsgID != nil {
			rmi = *mb.FOriginMsgID
		}
		var li = listItemForMessage(lmi, rmi, env)
		cio.ListMessage(li)
		// If we have oenv/omsg, it's a delivery receipt to be sent.
		if oenv != nil {
			if err = c.sendMessage(lmi+".DR", oenv, omsg); err != nil {
				return false, err
			}
		}
	}
KILL:
	// Kill the message from the BBS, unless it's a bulletin.  Note that we
	// intentionally don't check for a sigint here; once we've sent the
	// delivery receipt, we definitely want to kill the message.
	if area == "" {
		cio.Status("Removing message %d from BBS...", msgnum)
		if err = c.conn.Kill(msgnum); err != nil {
			return false, fmt.Errorf("JNOS kill %d: %s", msgnum, err)
		}
	}
	return false, nil
}

func (c *connection) receiveImmediates() (err error) {
	var msgnums []int
	if msgnums, err = c.immediateMessageNumbers(); err != nil {
		return fmt.Errorf("get list of immediate messages: %s", err)
	}
	for _, msgnum := range msgnums {
		var done bool
		if done, err = c.receiveMessage("", msgnum); err != nil {
			return fmt.Errorf("receive immediate message %d: %s", msgnum, err)
		}
		if done {
			return fmt.Errorf("message number %d went missing", msgnum)
		}
	}
	return nil
}

// immediateMessageNumbers returns the list of message numbers of immediate
// messages in the current mailbox.
func (c *connection) immediateMessageNumbers() (nums []int, err error) {
	if c.checkSigInt() {
		return nil, ErrInterrupted
	}
	cio.Status("Getting list of messages in inbox...")
	list, err := c.conn.List("")
	if err != nil {
		return nil, fmt.Errorf("JNOS list: %s", err)
	}
	for _, li := range list.Messages {
		if _, _, handling, _, _ := message.DecodeSubject(li.SubjectPrefix); handling == "I" {
			nums = append(nums, li.Number)
		}
	}
	return nums, nil
}

// receiveBulletins retrieves new bulletins from the specified areas.
func (c *connection) receiveBulletins() (err error) {
	for area, bc := range c.areas {
		var msgnums []int
		if msgnums, err = c.bulletinsToFetch(area, c.haveBulletins[area]); err != nil {
			return fmt.Errorf("determine bulletins to fetch from %s: %s", area, err)
		}
		for _, msgnum := range msgnums {
			var done bool
			if done, err = c.receiveMessage(area, msgnum); err != nil {
				return fmt.Errorf("receive message %d in %s: %s", msgnum, area, err)
			}
			if done {
				return fmt.Errorf("message number %d went missing", msgnum)
			}
		}
		if bc.Frequency == 0 {
			delete(config.C.Bulletins, area)
		} else {
			bc.LastCheck = time.Now()
		}
	}
	return nil
}

// bulletinsToFetch returns the list of message numbers of bulletin messages in
// the specified area that have not already been retrieved.
func (c *connection) bulletinsToFetch(area string, have map[string]bool) (nums []int, err error) {
	var to, areaonly string

	if c.checkSigInt() {
		return nil, ErrInterrupted
	}
	if idx := strings.IndexByte(area, '@'); idx >= 0 {
		to, areaonly = area[:idx], "ALL"+area[idx+1:]
	} else {
		areaonly = area
	}
	cio.Status("Moving to %s...", areaonly)
	if err := c.conn.SetArea(areaonly); err != nil {
		return nil, fmt.Errorf("JNOS area %s: %s", areaonly, err)
	}
	if c.checkSigInt() {
		return nil, ErrInterrupted
	}
	cio.Status("Getting list of messages for %s...", area)
	list, err := c.conn.List(to)
	if err != nil {
		return nil, fmt.Errorf("JNOS list %s: %s", area, err)
	}
	if list == nil {
		return nil, nil // no messages
	}
	for _, li := range list.Messages {
		if have == nil || !have[li.SubjectPrefix] {
			nums = append(nums, li.Number)
		}
	}
	return nums, nil
}

// checkSigInt returns whether an interrupt signal has been received.
func (c *connection) checkSigInt() (seen bool) {
	for {
		select {
		case <-c.sigintch:
			seen = true
		default:
			return
		}
	}
}

// drainSigInt stops trapping interrupt signals, drains the signal channel
// buffer, and closes the channel.
func (c *connection) drainSigInt() {
	var done bool
	signal.Reset(os.Interrupt)
	for !done {
		select {
		case <-c.sigintch:
			// no op
		default:
			done = true
		}
	}
	close(c.sigintch)
	c.sigintch = nil
}
