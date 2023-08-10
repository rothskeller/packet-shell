package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dumpCmd)
}

var dumpCmd = &cobra.Command{
	Use:                   "dump «message-id»",
	DisableFlagsInUseLine: true,
	SuggestFor:            []string{"raw"},
	Short:                 "Show a message in encoded form",
	Long: `
The "dump" command displays a message in its PackItForms- and RFC-5322-encoded
format, as it would be transmitted over the air.

«message-id» must be the local or remote message ID of the message to display.
It can be just the numeric part of the ID if that is unique.
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			lmi string
			fh  *os.File
		)
		if lmi, err = expandMessageID(args[0], true); err != nil {
			return err
		}
		if fh, err = os.Open(lmi + ".txt"); err != nil {
			return fmt.Errorf("reading %s: %s", lmi, err)
		}
		io.Copy(os.Stdout, fh)
		fh.Close()
		return nil
	},
}
