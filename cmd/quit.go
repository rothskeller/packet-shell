package cmd

import (
	"os"

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
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(0)
	},
}
