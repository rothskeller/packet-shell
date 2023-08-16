package cmd

import (
	"fmt"
	"runtime/debug"
)

const versionSlug = `Show "packet" version number`
const versionHelp = `
usage: packet version

The "version" command displays the version number of the "packet" command.
`

func cmdVersion([]string) error {
	if info, ok := debug.ReadBuildInfo(); ok {
		fmt.Printf("%s version %s de KC6RSC\n", info.Main.Path, info.Main.Version)
	} else {
		fmt.Println("github.com/rothskeller/packet-shell version (unknown) de KC6RSC")
	}
	return nil
}
