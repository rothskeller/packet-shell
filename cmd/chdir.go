package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rothskeller/packet/message"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(chdirCmd)
}

var chdirCmd = &cobra.Command{
	Use:                   "cd [«directory»]",
	DisableFlagsInUseLine: true,
	Aliases:               []string{"chdir", "md", "mkdir", "pwd"},
	Args:                  cobra.MaximumNArgs(1),
	Short:                 "Switch to a different incident directory",
	Long: `
The "cd" (or "chdir") command switches to a different incident directory.
When run without arguments or as "pwd", it displays the current incident
directory path.  When run as "mkdir" or "md", it will create the specified
directory and switch into it.`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		if cmd.CalledAs() == "pwd" && len(args) == 1 {
			return errors.New("cannot specify a directory with pwd")
		}
		if len(args) == 0 {
			var dir string
			if dir, err = os.Getwd(); err != nil {
				return err
			}
			if nd, err := filepath.EvalSymlinks(dir); err == nil {
				dir = nd
			}
			fmt.Println(dir)
			return nil
		}
		if err = os.Chdir(args[0]); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil {
			safeDir = safeDirectory()
			return nil
		}
		if ca := cmd.CalledAs(); ca != "md" && ca != "mkdir" {
			if !term.Human() {
				return err
			}
			var create = "No"
			_, err = term.EditField(message.NewRestrictedField(&message.Field{
				Label:    "Create directory?",
				Value:    &create,
				Choices:  message.Choices{"Yes", "No"},
				EditHelp: `The specified directory does not exist.  If you answer "Yes", it will be created.`,
			}), 0)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(create, "Y") {
				return nil
			}
		}
		if err = os.Mkdir(args[0], 0777); err != nil {
			return fmt.Errorf("creating directory: %s", err)
		}
		if err = os.Chdir(args[0]); err != nil {
			return fmt.Errorf("changing into created directory: %s", err)
		}
		// No point in checking safe directory.  We can't have created
		// an unsafe one.
		return nil
	},
}
