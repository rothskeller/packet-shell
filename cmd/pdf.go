package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet-shell/config"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/spf13/pflag"
)

const pdfSlug = `Open message in PDF form in system viewer`
const pdfHelp = `
usage: packet pdf «message-id»

The "pdf" command renders the message in PDF format if it isn't already rendered, and then opens it in the system PDF viewer.  The message must be of a type that supports PDF rendering (i.e., a PackItForms form), and PDF rendering support must have been built into the program.  The rendered PDF is stored at «local-message-id».pdf, with a symbolic link from «remote-message-id».pdf if a remote message ID is known.

«message-id» must be the local or remote message ID of the message to display.  It can be just the numeric part of the ID if that is unique.
`

func cmdPDF(args []string) (err error) {
	var (
		lmi   string
		msg   message.Message
		txtFI os.FileInfo
		pdfFI os.FileInfo
		open  *exec.Cmd
	)
	var flags = pflag.NewFlagSet("pdf", pflag.ContinueOnError)
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"pdf"})
	} else if err != nil {
		cio.Error(err.Error())
		return usage(pdfHelp)
	}
	if len(args) != 1 {
		return usage(pdfHelp)
	}
	if lmi, err = expandMessageID(args[0], true); err != nil {
		return err
	}
	// Check to be sure that the PDF is newer than the TXT.  If not,
	// it needs to be regenerated.
	if txtFI, err = os.Stat(lmi + ".txt"); err != nil {
		return err
	}
	if pdfFI, err = os.Stat(lmi + ".pdf"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if pdfFI == nil || pdfFI.ModTime().Before(txtFI.ModTime()) {
		// PDF needs to be regenerated.
		if _, msg, err = incident.ReadMessage(lmi); err != nil {
			return fmt.Errorf("reading %s: %s", lmi, err)
		}
		if err = msg.RenderPDF(lmi + ".pdf"); err != nil {
			return fmt.Errorf("rendering PDF: %s", err)
		}
	}
	switch runtime.GOOS {
	case "windows":
		open = exec.Command("cmd.exe", "/C", lmi+".pdf")
	case "darwin":
		open = exec.Command("open", lmi+".pdf")
	default:
		open = exec.Command("xdg-open", lmi+".pdf")
	}
	if err := open.Start(); err != nil {
		return fmt.Errorf("starting PDF viewer: %s", err)
	}
	go func() { open.Wait() }()
	if config.C.Unread[lmi] {
		config.C.Unread[lmi] = false
		config.SaveConfig()
	}
	return nil

}
