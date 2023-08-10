package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(quitCmd)
}

var quitCmd = &cobra.Command{
	Use:                   "quit",
	DisableFlagsInUseLine: true,
	Aliases:               []string{"q", "exit"},
	Short:                 "Quit the packet shell",
	Args:                  cobra.NoArgs,
	RunE:                  func(*cobra.Command, []string) error { return ErrQuit },
}
