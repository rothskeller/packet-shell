package cmd

import (
	"sort"

	"github.com/rothskeller/packet/message"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(typesHelp)
	typesHelp.SetHelpFunc(func(*cobra.Command, []string) {
		var tags = make([]string, 0, len(message.RegisteredTypes))
		var taglen int
		for tag := range message.RegisteredTypes {
			if msg := message.Create(tag); msg == nil || !msg.Editable() {
				continue
			}
			tags = append(tags, tag)
			aliaslen := len(aliases[tag])
			if aliaslen != 0 {
				aliaslen += 3
			}
			taglen = max(taglen, len(tag)+aliaslen)
		}
		sort.Strings(tags)
		for _, tag := range tags {
			var name = message.RegisteredTypes[tag].Name
			if alias := aliases[tag]; alias != "" {
				tag += " (" + alias + ")"
			}
			term.ShowNameValue(tag, name, taglen)
		}
		term.EndNameValueList()
	})
}

var typesHelp = &cobra.Command{
	Use:   "types",
	Short: "list of supported message types",
}
