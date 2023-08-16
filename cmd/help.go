package cmd

import (
	"io"
	"os"
	"strings"

	"github.com/rothskeller/packet-shell/cio"
)

const helpSlug = `Print help for packet commands or topics`
const topHelp = `
The "packet" command provides multiple commands for handling packet radio messages.  When invoked with a command on the command line, it runs that command.  When invoked without any arguments, it starts a shell that allows running multiple commands without the "packet" prefix on each.

Available commands include:
  bulletins  ⇥` + bulletinsSlug + `
  cd         ⇥` + chdirSlug + `
  connect    ⇥` + connectSlug + `
  delete     ⇥` + deleteSlug + `
  draft      ⇥` + draftSlug + `
  dump       ⇥` + dumpSlug + `
  edit       ⇥` + editSlug + `
  help       ⇥` + helpSlug + `
  ics309     ⇥` + ics309Slug + `
  list       ⇥` + listSlug + `
  new        ⇥` + newSlug + `
  pdf        ⇥` + pdfSlug + `
  queue      ⇥` + queueSlug + `
  quit       ⇥` + quitSlug + `
  set        ⇥` + setSlug + `
  show       ⇥` + showSlug + `
  version    ⇥` + versionSlug + `
For help on a command, run "packet help «command»".

Additional help is available on the following topics:
  config     ⇥` + configSlug + `
  files      ⇥` + filesSlug + `
  script     ⇥` + scriptSlug + `
  types      ⇥` + typesSlug + `
For these topics, run "packet help «topic»".

The "packet" command was written by Steve Roth KC6RSC.  Source code, licensing details, and bug tracker are at github.com/rothskeller/packet-shell.
`

func cmdHelp(args []string) (err error) {
	var helpText string

	if len(args) != 0 {
		switch args[0] {
		case "b", "bull", "bulletin", "bulletins":
			helpText = bulletinsHelp
		case "cd", "chdir", "md", "mkdir", "pwd":
			helpText = chdirHelp
		case "config":
			helpText = configHelp
		case "c", "connect":
			helpText = connectHelp
		case "delete":
			helpText = deleteHelp
		case "draft":
			helpText = draftHelp
		case "dump":
			helpText = dumpHelp
		case "e", "edit":
			helpText = editHelp
		case "files":
			helpText = filesHelp
		case "309", "ics309":
			helpText = ics309Help
		case "l", "list":
			helpText = listHelp
		case "n", "new":
			helpText = newHelp
		case "pdf":
			helpText = pdfHelp
		case "queue":
			helpText = queueHelp
		case "q", "quit", "exit":
			helpText = quitHelp
		case "script":
			helpText = scriptHelp
		case "set":
			helpText = setHelp
		case "s", "show":
			helpText = showHelp
		case "types":
			typesHelp() // special case, computed content
			return nil
		case "version":
			helpText = versionHelp
		default:
			cio.Error("no such command or help topic %q", args[0])
		}
	}
	if helpText == "" {
		helpText = topHelp
	}
	helpText = strings.TrimLeft(helpText, "\n") // Allows newline after `
	io.WriteString(os.Stdout, cio.WrapText(helpText))
	return nil
}

func usage(help string) error {
	help = strings.TrimLeft(help, "\n")
	if idx := strings.Index(help, "\n\n"); idx > 0 {
		help = help[:idx]
	}
	return ErrUsage(help)
}

type ErrUsage string

func (e ErrUsage) Error() string { return string(e) }
