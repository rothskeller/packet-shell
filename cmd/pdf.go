//go:build packetpdf

package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/rothskeller/packet/message"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(pdfCmd)
}

var pdfCmd = &cobra.Command{
	Use:                   "pdf «message-id»",
	DisableFlagsInUseLine: true,
	Short:                 "Open message in PDF form in system viewer",
	Long: `The "pdf" command renders the message in PDF format if it isn't already
rendered, and then opens it in the system PDF viewer.  The message must be of
a type that supports PDF rendering (i.e., a PackItForms form).  The rendered
PDF is stored at «local-message-id».pdf, with a symbolic link from
«remote-message-id».pdf if a remote message ID is known.

«message-id» must be the local or remote message ID of the message to display.
It can be just the numeric part of the ID if that is unique.
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			lmi   string
			msg   message.Message
			txtFI os.FileInfo
			pdfFI os.FileInfo
			open  *exec.Cmd
		)
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
		return nil
	},
}
