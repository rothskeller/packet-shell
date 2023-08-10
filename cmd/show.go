package cmd

import (
	"fmt"
	"strings"

	"github.com/rothskeller/packet-cmd/config"
	"github.com/rothskeller/packet/envelope"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(showCmd)
}

var showCmd = &cobra.Command{
	Use:                   "show «message-id»|config [«field-name»]",
	DisableFlagsInUseLine: true,
	Aliases:               []string{"s"},
	SuggestFor:            []string{"print", "view"},
	Short:                 "Show a message, or a field of a message",
	Long: `
The "show" command displays a message in a two-column field-name / field-value
tabular format.  It can also display the value of a single field of the
message.

«message-id» must be the local or remote message ID of the message to display.
It can be just the numeric part of the ID if that is unique.  If the word
"config" is used, the "show" command shows the incident / activation settings.

«field-name» is an optional name of a single field to display.  It can be the
PackItForms tag for the field (including the trailing period, if any), or it
can be the full field name.  In interactive (--human) mode, it can be a
shortened version of the field name, such as "ocs" for "Operator Call Sign."
`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			lmi      string
			env      *envelope.Envelope
			msg      message.Message
			fields   []*message.Field
			field    *message.Field
			labellen int
		)
		if args[0] == "config" {
			lmi, msg = "config", &config.C
		} else {
			if lmi, err = expandMessageID(args[0], true); err != nil {
				return err
			}
			if env, msg, err = incident.ReadMessage(lmi); err != nil {
				return fmt.Errorf("reading %s: %s", lmi, err)
			}
			// Create artificial "fields" for the envelope data we want to
			// show.
			fields = append(fields, makeArtificialField("Message Type", strings.ToUpper(msg.Base().Type.Name[:1])+msg.Base().Type.Name[1:]))
			if env.IsReceived() {
				fields = append(fields, makeArtificialField("From", env.From))
				fields = append(fields, makeArtificialField("Sent", env.Date.Format("01/02/2006 15:04")))
				fields = append(fields, makeArtificialField("To", strings.Join(env.To, ", ")))
				fields = append(fields, makeArtificialField("Received", fmt.Sprintf("%s as %s", env.ReceivedDate.Format("01/02/2006 15:04"), lmi)))
			} else {
				if len(env.To) != 0 {
					fields = append(fields, makeArtificialField("To", strings.Join(env.To, ", ")))
				}
				if !env.Date.IsZero() {
					fields = append(fields, makeArtificialField("Sent", env.Date.Format("01/02/2006 15:04")))
				}
			}
		}
		// Add to them the actual message fields.
		fields = append(fields, msg.Base().Fields...)
		// If we were asked to show a single field — which may be one of
		// the artificial ones — do that.
		if len(args) == 2 {
			if field, err = expandFieldName(fields, args[1], term.Human()); err != nil {
				return err
			}
			// Get the value of the field.  If TableValue returns an
			// empty string, it could be that the user specified the
			// PIFOTag of a field that is normally suppressed
			// because its value is displayed by a separate
			// aggregator field.  In that case, we want to show the
			// unaggregated value anyway.
			value := field.TableValue(field)
			if value == "" && args[1] == field.PIFOTag && field.Value != nil {
				value = *field.Value
			}
			term.ShowNameValue(field.Label, value, 0)
			return nil
		}
		j := 0
		for _, f := range fields {
			if f.TableValue(f) != "" {
				if len(f.Label) > labellen {
					labellen = len(f.Label)
				}
				fields[j], j = f, j+1
			}
		}
		fields = fields[:j]
		for _, f := range fields {
			term.ShowNameValue(f.Label, f.TableValue(f), labellen)
		}
		term.EndNameValueList()
		return nil
	},
}

func makeArtificialField(label, value string) (f *message.Field) {
	return message.AddFieldDefaults(&message.Field{Label: label, Value: &value})
}
