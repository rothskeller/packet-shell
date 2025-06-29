package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet/message"
	"github.com/spf13/pflag"
)

const (
	chdirSlug = `Switch to a different incident directory`
	chdirHelp = `
usage: cd [«directory»]
       mkdir «directory»
       pwd

When run without an argument, the "cd" (or "chdir" or "pwd") command displays the current incident directory path.

When run with an argument, the "cd" (or "chdir", "mkdir", or "md") command switches to the named incident directory.  If the directory does not exist, it will be created if the command name was "mkdir" or "md".  If the command name was "cd" or "chdir" and the command in running in interactive (--no-script) mode, the user is asked whether to create it.
`
)

func cmdChdir(cmdname string, args []string) (err error) {
	flags := pflag.NewFlagSet("chdir", pflag.ContinueOnError)
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"chdir"})
	} else if err != nil {
		cio.Error("%s", err.Error())
		return usage(chdirHelp)
	}
	switch cmdname {
	case "pwd":
		if len(args) != 0 {
			return usage(chdirHelp)
		}
	case "cd", "chdir":
		if len(args) > 1 {
			return usage(chdirHelp)
		}
	case "md", "mkdir":
		if len(args) != 1 {
			return usage(chdirHelp)
		}
	}
	if len(args) == 0 {
		return doPwd()
	}
	return doChdir(args[0], cmdname == "md" || cmdname == "mkdir")
}

func doPwd() (err error) {
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

func doChdir(dir string, automake bool) (err error) {
	if err = os.Chdir(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		safeDir = safeDirectory()
		return nil
	}
	if !automake {
		if !cio.InputIsTerm || !cio.OutputIsTerm {
			return err
		}
		create := "No"
		_, err = cio.EditField(message.NewRestrictedField(&message.Field{
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
	if err = os.Mkdir(dir, 0777); err != nil {
		return fmt.Errorf("creating directory: %s", err)
	}
	if err = os.Chdir(dir); err != nil {
		return fmt.Errorf("changing into created directory: %s", err)
	}
	// No point in checking safe directory.  We can't have created an unsafe
	// one.
	return nil
}
