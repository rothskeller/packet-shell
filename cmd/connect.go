package cmd

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/rothskeller/packet-cmd/config"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/jnos"
	"github.com/rothskeller/packet/jnos/kpc3plus"
	"github.com/rothskeller/packet/jnos/telnet"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/delivrcpt"
	"github.com/rothskeller/packet/xscmsg/readrcpt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(connectCmd)
	connectCmd.Flags().BoolP("send", "s", false, "send queued messages")
	connectCmd.Flags().BoolP("receive", "r", false, "receive incoming messages")
	connectCmd.Flags().BoolP("immediate", "i", false, "immediate messages only")
}

var ErrInterrupted = errors.New("connection interrupted by Ctrl-C")

type connection struct {
	tosend        []string
	rcvlevel      int
	subjectToLMI  map[string]string
	areas         map[string]*config.BulletinConfig
	haveBulletins map[string]map[string]bool
	sigintch      chan os.Signal
	conn          *jnos.Conn
}

var connectCmd = &cobra.Command{
	Use:                   "connect [--send] [--receive] [--immediate]",
	DisableFlagsInUseLine: true,
	Aliases:               []string{"c"},
	SuggestFor:            []string{"send", "receive"},
	Short:                 "Connect to the BBS to send and/or receive messages",
	Long: `
The "connect" command makes a connection to the BBS and sends and/or receives
messages.  With the --send flag, it sends queued outgoing messages; with the
--receive flag, it receives incoming messages; with both or neither, it does
both.  With the --immediate flag, only immediate messages are sent and/or
received.

When receiving messages without the --immediate flag, any scheduled bulletin
checks are performed as well.  (See the "packet bulletins" command for
scheduling of bulletin checks.)
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			sendlevel int
			conn      connection
		)
		if f, _ := cmd.Flags().GetBool("send"); f {
			sendlevel = 1
		}
		if f, _ := cmd.Flags().GetBool("receive"); f {
			conn.rcvlevel = 1
		}
		if sendlevel == 0 && conn.rcvlevel == 0 {
			sendlevel, conn.rcvlevel = 1, 1
		}
		if f, _ := cmd.Flags().GetBool("immediate"); f {
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
			term.Confirm("Nothing to send.")
			return nil
		}
		if !haveConnectConfig() && term.Human() {
			term.Confirm("Please provide necessary configuration settings for connection:")
			saveTerm := term
			rootCmd.SetArgs([]string{"edit", "config"})
			err = rootCmd.Execute()
			term.Close()
			term = saveTerm
			if err != nil {
				return err
			}
		}
		if !haveConnectConfig() {
			term.Error("missing necessary configuration settings")
		}
		// Run the connection.
		defer term.Status("")
		if err := conn.run(); err != nil {
			return err
		}
		// Save the configuration with new LastCheck times for the bulletin
		// areas.
		if len(conn.areas) != 0 {
			config.SaveConfig()
		}
		return nil
	},
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

// doConnect connects to the BBS and performs the desired operations.  It sends
// all of the messages whose filenames are in the tosend array.  If rcvlevel is
// 2, it receives immediate messages.  If rcvlevel is 1, it receives all
// incoming messages.  If rcvlevel is 0, it does not receive any messages.  All
// bulletin areas listed in areas are checked for new bulletins.
func (c *connection) run() (err error) {
	var (
		mailbox string
		log     *os.File
	)
	// Intercept ^C so we can close the connection gracefully.
	c.sigintch = make(chan os.Signal, 10)
	signal.Notify(c.sigintch, os.Interrupt)
	defer c.drainSigInt()
	// Append to the log file.
	if log, err = os.OpenFile("packet.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666); err != nil {
		return err
	}
	defer log.Close()
	// Connect to the BBS.
	if config.C.TacCall != "" {
		mailbox = config.C.TacCall
	} else {
		mailbox = config.C.OpCall
	}
	term.Status("Connecting to %s@%s...", mailbox, config.C.BBS)
	if strings.IndexByte(config.C.BBSAddress, ':') >= 0 { // internet connection
		c.conn, err = telnet.Connect(config.C.BBSAddress, mailbox, config.C.Password, log)
	} else { // radio connection
		c.conn, err = kpc3plus.Connect(config.C.SerialPort, config.C.BBSAddress, mailbox, config.C.OpCall, log)
	}
	if err != nil {
		return fmt.Errorf("JNOS connect: %s", err)
	}
	defer func() {
		term.Status("\r\033[KClosing connection...")
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
	term.EndMessageList("No messages sent or received.")
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
		env.From = (&mail.Address{Name: config.C.TacName, Address: strings.ToLower(config.C.TacCall + "@" + config.C.BBS + ".ampr.org")}).String()
	} else {
		env.From = (&mail.Address{Name: config.C.OpName, Address: strings.ToLower(config.C.OpCall + "@" + config.C.BBS + ".ampr.org")}).String()
	}
	env.Date = time.Now()
	msg.SetOperator(config.C.OpCall, config.C.OpName, false)
	body := msg.EncodeBody()
	if strings.HasSuffix(filename, ".DR") {
		term.Status("Sending delivery receipt for %s...", filename[:len(filename)-3])
	} else {
		term.Status("Sending %s...", filename)
	}
	if err = c.conn.Send(env.SubjectLine, env.RenderBody(body), env.To...); err != nil {
		return fmt.Errorf("JNOS send: %s", err)
	}
	if strings.HasSuffix(filename, ".DR") {
		if err = incident.SaveReceipt(filename[:len(filename)-3], env, msg); err != nil {
			return fmt.Errorf("save receipt %s: %s", filename, err)
		}
	} else {
		if err = incident.SaveMessage(filename, "", env, msg); err != nil {
			return fmt.Errorf("save message %s: %s", filename, err)
		}
	}
	if !strings.HasSuffix(filename, ".DR") {
		term.ListMessage(listItemForMessage(filename, "", env))
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
// when appropriate.  It returns false, true if everything's OK; true, true if
// no message with the specified number exists, and false, false if some other
// error occurs (in which case an error message will have been printed).
func (c *connection) receiveMessage(area string, msgnum int) (done bool, err error) {
	if c.checkSigInt() {
		return false, ErrInterrupted
	}
	// Read the message.
	if area != "" {
		term.Status("Reading message %d in %s...", msgnum, area)
	} else {
		term.Status("Reading message %d...", msgnum)
	}
	raw, err := c.conn.Read(msgnum)
	if err != nil {
		return false, fmt.Errorf("JNOS read %d: %s", msgnum, err)
	}
	if raw == "" {
		return true, nil
	}
	// Parse the message.
	env, body, err := envelope.ParseRetrieved(raw, config.C.BBS, area)
	if err != nil {
		return false, fmt.Errorf("parse retrieved message: %s", err)
	}
	if env.Autoresponse {
		term.Confirm("NOTE: ignoring message %d, which is an autoresponse", msgnum)
		return false, nil
	}
	msg := message.Decode(env.SubjectLine, body)
	// If it's a receipt, it's handled specially.
	switch msg.(type) {
	case *delivrcpt.DeliveryReceipt, *readrcpt.ReadReceipt:
		if err = c.recordReceipt(env, msg); err != nil {
			return false, fmt.Errorf("record receipt: %s", err)
		}
		term.Status("Removing message %d from BBS...", msgnum)
		if err = c.conn.Kill(msgnum); err != nil {
			return false, fmt.Errorf("JNOS kill %d: %s", msgnum, err)
		}
		return false, nil
	}
	// Assign a local message ID.  Put it, and the opcall/opname, into the
	// message if it has fields for it.
	lmi := incident.UniqueMessageID(config.C.MessageID)
	if mb := msg.Base(); mb.FDestinationMsgID != nil {
		*mb.FDestinationMsgID = lmi
	}
	msg.SetOperator(config.C.OpCall, config.C.OpName, true)
	// Save the message.
	var rmi string
	if b := msg.Base(); b.FOriginMsgID != nil {
		rmi = *b.FOriginMsgID
	}
	if err = incident.SaveMessage(lmi, rmi, env, msg); err != nil {
		return false, fmt.Errorf("save received %s: %s", lmi, err)
	}
	term.ListMessage(listItemForMessage(lmi, rmi, env))
	if area != "" {
		// bulletin: no delivery receipt, no kill message
		return false, nil
	}
	// Send delivery receipt.
	if err = c.sendDeliveryReceipt(lmi, env); err != nil {
		return false, fmt.Errorf("send delivery receipt for %s: %s", lmi, err)
	}
	// Kill message from BBS.  Note that we intentionally don't check for
	// a sigint here; once we've sent the delivery receipt, we definitely
	// want to kill the message.
	term.Status("Removing message %d from BBS...", msgnum)
	if err = c.conn.Kill(msgnum); err != nil {
		return false, fmt.Errorf("JNOS kill %d: %s", msgnum, err)
	}
	return false, nil
}

// recordReceipt matches a received receipt with the corresponding outgoing
// message.
func (c *connection) recordReceipt(env *envelope.Envelope, msg message.Message) (err error) {
	var (
		subject string
		lmi     string
		ext     string
		rmi     string
	)
	switch msg := msg.(type) {
	case *delivrcpt.DeliveryReceipt:
		subject = msg.MessageSubject
		ext = ".DR"
		rmi = msg.LocalMessageID
	case *readrcpt.ReadReceipt:
		subject = msg.MessageSubject
		ext = ".RR"
	}
	if subject != "" {
		lmi = c.subjectToLMI[subject]
	}
	if lmi == "" {
		term.Confirm("NOTE: discarding delivery receipt for unknown message with subject %q", subject)
		return nil
	}
	if _, err = os.Stat(lmi + ext); !errors.Is(err, os.ErrNotExist) {
		term.Confirm("NOTE: discarding duplicate receipt for %s", lmi)
		return nil
	}
	if err = incident.SaveReceipt(lmi, env, msg); err != nil {
		return fmt.Errorf("save receipt for %s: %s", lmi, err)
	}
	if rmi == "" {
		return nil // read receipt, nothing more to do
	}
	if env, msg, err = incident.ReadMessage(lmi); err != nil {
		return fmt.Errorf("add RMI: read message %s: %s", lmi, err)
	}
	if mb := msg.Base(); mb.FDestinationMsgID != nil {
		*mb.FDestinationMsgID = rmi
		if err = incident.SaveMessage(lmi, rmi, env, msg); err != nil {
			return fmt.Errorf("add RMI: save message %s: %s", lmi, err)
		}
	}
	li := listItemForMessage(lmi, rmi, env)
	li.Flag = "HAVE RCPT"
	term.ListMessage(li)
	return nil
}

// sendDeliveryReceipt sends (and saves) a delivery receipt for the message.
func (c *connection) sendDeliveryReceipt(lmi string, renv *envelope.Envelope) (err error) {
	var (
		denv envelope.Envelope
		dr   *delivrcpt.DeliveryReceipt
	)
	dr = delivrcpt.New()
	dr.LocalMessageID = lmi
	dr.DeliveredTime = time.Now().Format("01/02/2006 15:04")
	dr.MessageSubject = renv.SubjectLine
	dr.MessageTo = strings.Join(renv.To, ", ")
	denv.SubjectLine = dr.EncodeSubject()
	denv.To = []string{renv.From}
	return c.sendMessage(lmi+".DR", &denv, dr)
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
	term.Status("Getting list of messages in inbox...")
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
		bc.LastCheck = time.Now()
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
		to, areaonly = area[:idx], area[idx+1:]
	} else {
		areaonly = area
	}
	term.Status("Moving to %s...", areaonly)
	if err := c.conn.SetArea(areaonly); err != nil {
		return nil, fmt.Errorf("JNOS area %s: %s", areaonly, err)
	}
	if c.checkSigInt() {
		return nil, ErrInterrupted
	}
	term.Status("Getting list of messages for %s...", area)
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
