package cmd

import "github.com/spf13/cobra"

func init() {
	rootCmd.AddCommand(scriptHelp)
}

var scriptHelp = &cobra.Command{
	Use:   "script",
	Short: "explanation of --script / --no-script behaviors",
	Long: `The "packet" commands vary their input and output behavior to suit both
scripted and human-interactive use.  Normally, they target human-interactive
use when both standard input and standard output are a terminal device; they
target scripted use when either standard input or standard input is something
else (e.g., redirected to/from a file, or piped to/from another program).
You can override the default with the --script or --no-script flag to any
"packet" command.

In human mode, informative messages are printed.  These include status updates
on the progress of a long operation, and confirmations when a requested action
has been completed.  In scripted mode, these messages are not printed.

In human mode, all output goes to the standard output device.  In scripted
mode, all output goes to the standard output device except error messages,
which go to the standard error device instead.

In human mode, tabular output is lined up in columns for easy reading, data
are formatted to fit the terminal width, and colors are used to highlight row
and column headers when the terminal supports that.  In scripted mode,
tabular output is printed in CSV format with no special formatting.

In human mode, value entry for message fields and configuration settings uses
an input editor.  If the terminal supports it, this editor allows the use of
arrow keys, autocomplete, choice selection, colorization, etc.  Even if the
terminal does not support these things, the human mode displays the field's
previous value before reading the new value, accepts an empty entry as
meaning "no change", prompts for retries after entering invalid values, and
allows access to online help.  In scripted mode, the entire standard input is
read up to the end of file and used as the value being entered, with no
editing capability.  Note that because of this, the commands that allow
editing of multiple values together (edit, new, reply, resend) do not work in
scripted mode.
`,
}
