package shell

import (
	"errors"
	"fmt"
	"io"
	"net/mail"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/jnos"
	"github.com/rothskeller/packet/jnos/kpc3plus"
	"github.com/rothskeller/packet/jnos/telnet"
	"github.com/rothskeller/packet/message"
	"github.com/rothskeller/packet/xscmsg/delivrcpt"
	"github.com/rothskeller/packet/xscmsg/readrcpt"
)

// cmdConnect implements the connect command.
func cmdConnect(args []string, sendlevel, rcvlevel int) (ok bool) {
	var (
		tosend        []string
		areas         map[string]*bulletinConfig
		subjectToLMI  map[string]string
		haveBulletins map[string]map[string]bool
	)
	if sendlevel < 2 && rcvlevel < 2 && len(args) != 0 && (args[0] == "immediate" || args[0] == "i") {
		sendlevel, rcvlevel = sendlevel*2, rcvlevel*2
		args = args[1:]
	}
	if len(args) != 0 {
		fmt.Fprintf(os.Stderr, "usage: packet connect|send|receive [immediate]\n")
		return false
	}
	// If we're checking bulletins, make a map of the areas to check based
	// on time elapsed and requested frequency.
	if rcvlevel == 1 {
		areas = make(map[string]*bulletinConfig)
		for area, bc := range config.Bulletins {
			if time.Since(bc.LastCheck) >= bc.Frequency {
				areas[area] = bc
			}
		}
	}
	// Scan through all existing messages, gathering data that we will need
	// to handle the connection
	tosend, subjectToLMI, haveBulletins = preConnectScan(sendlevel, areas)
	// Do we have anything to do?
	if len(tosend) == 0 && rcvlevel == 0 && len(areas) == 0 {
		io.WriteString(os.Stdout, "NOTE: nothing to do, so not connecting\n")
		return true
	}
	if !requestConfig("bbs", "tnc", "operator", "tactical", "password", "msgid") {
		return false
	}
	// Run the connection.
	if !doConnect(tosend, rcvlevel, subjectToLMI, areas, haveBulletins) {
		return false
	}
	// Save the configuration with new LastCheck times for the bulletin
	// areas.
	if len(areas) != 0 {
		saveConfig()
	}
	return true
}

func helpConnect() {
	io.WriteString(os.Stdout, `The connection commands connect to a BBS to send and/or receive messages.
    usage: packet connect|send|receive [immediate]
The "send" command sends queued messages, the "receive" command receives
incoming messages and performs scheduled bulletin checks, and the "connect"
command does both.  These can be abbreviated "s", "r", and "c", respectively.
"sr" and "rs" are also accepted for performing both operations.
    When the "immediate" keyword is present, only messages with immediate
handling are sent and/or received, and no bulletin checks are performed.  The
"immediate" keyword can be abbreviated "i".  In abbreviated form, it can be
combined with the command word, so "si", "ri", "ci", "sri", and "rsi" are all
valid commands.
    For information about scheduling bulletin checks, see the help on the
"bulletins" command.
`)
}

var areaRE = regexp.MustCompile(`^(?:[A-Z][A-Z0-9]{0,7}@)?[A-Z][A-Z]{0,7}$`)

func cmdBulletin(args []string) bool {
	var frequency = time.Hour

	if config.Bulletins == nil {
		config.Bulletins = make(map[string]*bulletinConfig)
	}
	if len(args) > 1 {
		if f, err := time.ParseDuration(args[0]); err == nil && f >= 0 {
			frequency, args = f, args[1:]
		}
	}
	for _, arg := range args {
		arg = strings.ToUpper(arg)
		if !areaRE.MatchString(arg) {
			fmt.Fprintf(os.Stderr, "ERROR: %q is not a valid area name\n", arg)
			return false
		}
		if frequency == 0 {
			delete(config.Bulletins, arg)
		} else if b, ok := config.Bulletins[arg]; ok {
			b.Frequency = frequency
		} else {
			config.Bulletins[arg] = &bulletinConfig{Frequency: frequency}
		}
	}
	if len(args) == 0 {
		for area := range config.Bulletins {
			config.Bulletins[area].LastCheck = time.Time{}
		}
	}
	saveConfig()
	return cmdConnect(nil, 1, 1)
}

func helpBulletin() {
	io.WriteString(os.Stdout, `The "bulletins" (or "b") command schedules checks for bulletins.
    usage: packet bulletins
           packet bulletins [<frequency>] <area>...
The "bulletins" command updates the schedules for bulletin checks, and then
connects to the BBS and checks for new bulletins.
    In the first form, without arguments, the connection will check for new
bulletins in all areas that have a schedule, even if their next check isn't
due yet.
    In the second form, the schedules for the listed <area>s are changed to
have the specified <frequency> (default "1h"), and the connection will check
for new bulletins in those areas that are due for a check according to the new
schedule.
    Each <area> must be a bulletin area name (e.g., "XSCEVENT"), optionally
preceded by a recipient name and an at-sign (e.g., "XND@ALLXSC").
    The <frequency> specifies how frequently the listed bulletin areas should
be checked for new bulletins, formatted like "30m" or "2h15m".  Setting the
frequency to "0" removes the scheduled checks for the listed areas.
`)
}

// preConnectScan scans all existing messages gathering information needed prior
// to a connection.  It returns the list of messages to send, a map from subject
// line to LMI for already-received direct messages, and a map from bulletin
// area to the set of bulletin subjects already retrieved from that area.
func preConnectScan(sendlevel int, areas map[string]*bulletinConfig) (
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

var sigintch chan os.Signal

// doConnect connects to the BBS and performs the desired operations.  It sends
// all of the messages whose filenames are in the tosend array.  If rcvlevel is
// 2, it receives immediate messages.  If rcvlevel is 1, it receives all
// incoming messages.  If rcvlevel is 0, it does not receive any messages.  All
// bulletin areas listed in areas are checked for new bulletins.
func doConnect(
	tosend []string, rcvlevel int, subjectToLMI map[string]string, areas map[string]*bulletinConfig,
	haveBulletins map[string]map[string]bool,
) (ok bool) {
	var (
		mailbox string
		log     *os.File
		conn    *jnos.Conn
		list    lister
		err     error
	)
	// Intercept ^C so we can close the connection gracefully.
	sigintch = make(chan os.Signal, 10)
	signal.Notify(sigintch, os.Interrupt)
	defer drainSigInt()
	// Append to the log file.
	if log, err = os.OpenFile("packet.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		return false
	}
	defer log.Close()
	// Connect to the BBS.
	if config.TacCall != "" {
		mailbox = config.TacCall
	} else {
		mailbox = config.OpCall
	}
	fmt.Printf("Connecting to %s@%s...", mailbox, config.BBS)
	if strings.IndexByte(config.BBSAddress, ':') >= 0 { // internet connection
		conn, err = telnet.Connect(config.BBSAddress, mailbox, config.Password, log)
	} else { // radio connection
		conn, err = kpc3plus.Connect(config.SerialPort, config.BBSAddress, mailbox, config.OpCall, log)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KERROR: %s\n", err)
		return false
	}
	defer func() {
		io.WriteString(os.Stdout, "\r\033[KClosing connection...")
		if err = conn.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "\r\033[KERROR: close: %s\n", err)
			ok = false
		} else {
			io.WriteString(os.Stdout, "\r\033[K")
		}
	}()
	if !sendMessages(conn, &list, tosend) {
		return false
	}
	switch rcvlevel {
	case 1:
		if !receiveMessages(conn, &list, subjectToLMI) {
			return false
		}
	case 2:
		if !receiveImmediates(conn, &list) {
			return false
		}
	}
	if !receiveBulletins(conn, &list, areas, haveBulletins) {
		return false
	}
	if !list.seenOne {
		io.WriteString(os.Stdout, "\r\033[KNo messages sent or received.\n")
	}
	return true
}

// sendMessages sends the listed messages.
func sendMessages(conn *jnos.Conn, list *lister, lmis []string) bool {
	for _, lmi := range lmis {
		env, msg, err := incident.ReadMessage(lmi)
		if err != nil {
			return false
		}
		if !sendMessage(conn, list, lmi, env, msg) {
			return false
		}
	}
	return true
}

// sendMessage sends a single message.  It is used for both outgoing human
// messages and delivery receipts.
func sendMessage(conn *jnos.Conn, list *lister, filename string, env *envelope.Envelope, msg message.Message) bool {
	var err error

	if checkSigInt() {
		return false
	}
	if config.TacCall != "" {
		env.From = (&mail.Address{Name: config.TacName, Address: strings.ToLower(config.TacCall + "@" + config.BBS + ".ampr.org")}).String()
	} else {
		env.From = (&mail.Address{Name: config.OpName, Address: strings.ToLower(config.OpCall + "@" + config.BBS + ".ampr.org")}).String()
	}
	env.Date = time.Now()
	msg.SetOperator(config.OpCall, config.OpName, false)
	body := msg.EncodeBody()
	if strings.HasSuffix(filename, ".DR") {
		fmt.Printf("\r\033[KSending delivery receipt for %s...", filename[:len(filename)-3])
	} else {
		fmt.Printf("\r\033[KSending %s...", filename)
	}
	if err = conn.Send(env.SubjectLine, env.RenderBody(body), env.To...); err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KERROR: send: %s\n", err)
		return false
	}
	if strings.HasSuffix(filename, ".DR") {
		err = incident.SaveReceipt(filename[:len(filename)-3], env, msg)
	} else {
		err = incident.SaveMessage(filename, "", env, msg)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KERROR: %s\n", err)
		return false
	}
	io.WriteString(os.Stdout, "\r\033[K")
	if !strings.HasSuffix(filename, ".DR") {
		list.item(filename, "", env, false)
	}
	return true
}

// receiveMessages receives all messages in the current mailbox (i.e., the one
// we connected to).
func receiveMessages(conn *jnos.Conn, list *lister, subjectToLMI map[string]string) bool {
	var msgnum = 1
	for {
		done, ok := receiveMessage(conn, list, "", msgnum, subjectToLMI)
		if !ok {
			return false
		}
		if done {
			break
		}
		msgnum++
	}
	return true
}

// receiveMessage receives a single message with the specified number from the
// current area, and kills it from the server.  It also sends delivery receipts
// when appropriate.  It returns false, true if everything's OK; true, true if
// no message with the specified number exists, and false, false if some other
// error occurs (in which case an error message will have been printed).
func receiveMessage(conn *jnos.Conn, list *lister, area string, msgnum int, subjectToLMI map[string]string) (done, ok bool) {
	if checkSigInt() {
		return false, false
	}
	// Read the message.
	if area != "" {
		fmt.Printf("\r\033[KReading message %d in %s...", msgnum, area)
	} else {
		fmt.Printf("\r\033[KReading message %d...", msgnum)
	}
	raw, err := conn.Read(msgnum)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KERROR: read %d: %s\n", msgnum, err)
		return false, false
	}
	if raw == "" {
		return true, true
	}
	// Parse the message.
	env, body, err := envelope.ParseRetrieved(raw, config.BBS, area)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KERROR: read %d: %s\n", msgnum, err)
		return false, false
	}
	if env.Autoresponse {
		fmt.Printf("\r\033[KNOTE: ignoring message %d, which is an autoresponse\n", msgnum)
		return false, true
	}
	msg := message.Decode(env.SubjectLine, body)
	// If it's a receipt, it's handled specially.
	switch msg.(type) {
	case *delivrcpt.DeliveryReceipt, *readrcpt.ReadReceipt:
		if !recordReceipt(list, env, msg, subjectToLMI) {
			return false, false
		}
		fmt.Printf("\r\033[KRemoving message %d from BBS...", msgnum)
		if err := conn.Kill(msgnum); err != nil {
			fmt.Fprintf(os.Stderr, "\r\033[KERROR: kill %d: %s\n", msgnum, err)
			return false, false
		}
		return false, true
	}
	// Assign a local message ID.  Put it, and the opcall/opname, into the
	// message if it has fields for it.
	lmi := incident.UniqueMessageID(config.MessageID)
	if mb := msg.Base(); mb.FDestinationMsgID != nil {
		*mb.FDestinationMsgID = lmi
	}
	msg.SetOperator(config.OpCall, config.OpName, true)
	// Save the message.
	var rmi string
	if b := msg.Base(); b.FOriginMsgID != nil {
		rmi = *b.FOriginMsgID
	}
	if err = incident.SaveMessage(lmi, rmi, env, msg); err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KERROR: %s\n", err)
		return false, false
	}
	io.WriteString(os.Stdout, "\r\033[K")
	list.item(lmi, rmi, env, false)
	if area != "" {
		// bulletin: no delivery receipt, no kill message
		return false, true
	}
	// Send delivery receipt.
	if !sendDeliveryReceipt(conn, list, lmi, env) {
		return false, false
	}
	// Kill message from BBS.  Note that we intentionally don't check for
	// a sigint here; once we've sent the delivery receipt, we definitely
	// want to kill the message.
	fmt.Printf("\r\033[KRemoving message %d from BBS...", msgnum)
	if err := conn.Kill(msgnum); err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KERROR: kill %d: %s\n", msgnum, err)
		return false, false
	}
	return false, true
}

// recordReceipt matches a received receipt with the corresponding outgoing
// message.
func recordReceipt(list *lister, env *envelope.Envelope, msg message.Message, subjectToLMI map[string]string) bool {
	var (
		subject string
		lmi     string
		ext     string
		rmi     string
		err     error
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
		lmi = subjectToLMI[subject]
	}
	if lmi == "" {
		fmt.Fprintf(os.Stderr, "\r\033[KNOTE: discarding delivery receipt for unknown message with subject\n      %s\n", subject)
		return true // not an error worth stopping for
	}
	if _, err = os.Stat(lmi + ext); !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "\r\033[KNOTE: discarding duplicate receipt for %s\n", lmi)
		return true
	}
	if err = incident.SaveReceipt(lmi, env, msg); err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KERROR: %s\n", err)
		return false
	}
	if rmi == "" {
		return true // read receipt, nothing more to do
	}
	if env, msg, err = incident.ReadMessage(lmi); err != nil {
		return false
	}
	if mb := msg.Base(); mb.FDestinationMsgID != nil {
		*mb.FDestinationMsgID = rmi
		if err = incident.SaveMessage(lmi, rmi, env, msg); err != nil {
			fmt.Fprintf(os.Stderr, "\r\033[KERROR: %s\n", err)
			return false
		}
	}
	io.WriteString(os.Stdout, "\r\033[K")
	list.item(lmi, rmi, env, true)
	return true
}

// sendDeliveryReceipt sends (and saves) a delivery receipt for the message.
func sendDeliveryReceipt(conn *jnos.Conn, list *lister, lmi string, renv *envelope.Envelope) bool {
	var (
		denv envelope.Envelope
		dr   delivrcpt.DeliveryReceipt
	)
	dr.LocalMessageID = lmi
	dr.DeliveredTime = time.Now().Format("01/02/2006 15:04")
	dr.MessageSubject = renv.SubjectLine
	dr.MessageTo = strings.Join(renv.To, ", ")
	denv.SubjectLine = dr.EncodeSubject()
	denv.To = []string{renv.From}
	return sendMessage(conn, list, lmi+".DR", &denv, &dr)
}

func receiveImmediates(conn *jnos.Conn, list *lister) bool {
	msgnums, ok := immediateMessageNumbers(conn)
	if !ok {
		return false
	}
	for _, msgnum := range msgnums {
		done, fail := receiveMessage(conn, list, "", msgnum, nil)
		if fail {
			return false
		}
		if done {
			fmt.Fprintf(os.Stderr, "ERROR: message number %d went missing\n", msgnum)
			return false
		}
	}
	return true
}

// immediateMessageNumbers returns the list of message numbers of immediate
// messages in the current mailbox.
func immediateMessageNumbers(conn *jnos.Conn) (nums []int, ok bool) {
	if checkSigInt() {
		return nil, false
	}
	io.WriteString(os.Stdout, "\r\033[KGetting list of messages in inbox...")
	list, err := conn.List("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KERROR: list: %s\n", err)
		return nil, false
	}
	for _, li := range list.Messages {
		if _, _, handling, _, _ := message.DecodeSubject(li.SubjectPrefix); handling == "I" {
			nums = append(nums, li.Number)
		}
	}
	return nums, true
}

// receiveBulletins retrieves new bulletins from the specified areas.
func receiveBulletins(
	conn *jnos.Conn, list *lister, areas map[string]*bulletinConfig, haveBulletins map[string]map[string]bool,
) bool {
	for area, bc := range areas {
		msgnums, ok := bulletinsToFetch(conn, area, haveBulletins[area])
		if !ok {
			return false
		}
		for _, msgnum := range msgnums {
			done, ok := receiveMessage(conn, list, area, msgnum, nil)
			if !ok {
				return false
			}
			if done {
				fmt.Fprintf(os.Stderr, "ERROR: message number %d went missing\n", msgnum)
				return false
			}
		}
		bc.LastCheck = time.Now()
	}
	return true
}

// bulletinsToFetch returns the list of message numbers of bulletin messages in
// the specified area that have not already been retrieved.
func bulletinsToFetch(conn *jnos.Conn, area string, have map[string]bool) (nums []int, ok bool) {
	var to, areaonly string

	if checkSigInt() {
		return nil, false
	}
	if idx := strings.IndexByte(area, '@'); idx >= 0 {
		to, areaonly = area[:idx], area[idx+1:]
	} else {
		areaonly = area
	}
	fmt.Printf("\r\033[KMoving to %s...", areaonly)
	if err := conn.SetArea(areaonly); err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KERROR: change area: %s\n", err)
		return nil, false
	}
	if checkSigInt() {
		return nil, false
	}
	fmt.Printf("\r\033[KGetting list of messages for %s...", area)
	list, err := conn.List(to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KERROR: list: %s\n", err)
		return nil, false
	}
	if list == nil {
		return nil, true // no messages
	}
	for _, li := range list.Messages {
		if have == nil || !have[li.SubjectPrefix] {
			nums = append(nums, li.Number)
		}
	}
	return nums, true
}

// checkSigInt returns whether an interrupt signal has been received.
func checkSigInt() (seen bool) {
	for {
		select {
		case <-sigintch:
			seen = true
		default:
			return
		}
	}
}

// drainSigInt stops trapping interrupt signals, drains the signal channel
// buffer, and closes the channel.
func drainSigInt() {
	var done bool
	signal.Reset(os.Interrupt)
	for !done {
		select {
		case <-sigintch:
			// no op
		default:
			done = true
		}
	}
	close(sigintch)
	sigintch = nil
}
