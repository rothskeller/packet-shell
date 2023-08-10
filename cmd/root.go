package cmd

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rothskeller/packet-cmd/terminal"
	"github.com/rothskeller/packet/incident"
	"github.com/rothskeller/packet/message"

	"github.com/spf13/cobra"
)

var term, saveTerm terminal.Terminal

var ErrQuit = errors.New("quit requested")

func init() {
	rootCmd.PersistentFlags().Bool("script", false, "script-friendly input and output")
	rootCmd.PersistentFlags().Bool("no-script", false, "human-friendly input and output")
	rootCmd.PersistentFlags().MarkHidden("no-script")
	rootCmd.MarkFlagsMutuallyExclusive("script", "no-script")
	rootCmd.RunE = func(*cobra.Command, []string) (err error) {
		if !term.Human() {
			Execute([]string{"help"})
			return
		}
		for {
			var (
				line string
				args []string
			)
			if line, err = term.ReadCommand(); err != nil {
				if err == io.EOF {
					line = "quit"
				} else {
					return err
				}
			}
			args = tokenizeLine(line)
			rootCmd.SetArgs(args)
			saveTerm = term
			if err = rootCmd.Execute(); err != nil && err != ErrQuit {
				term.Error(err.Error())
			}
			term.Close()
			term, saveTerm = saveTerm, nil
			if err == ErrQuit {
				return nil
			}
		}
	}
}

var rootCmd = &cobra.Command{
	Use:                   "packet",
	DisableFlagsInUseLine: true,
	Short:                 "Packet radio message handler",
	Long: `The "packet" command provides multiple commands for handling packet radio
messages.  When invoked with a command on the command line, it runs that
command.  When invoked without any arguments, it starts a shell that allows
running multiple commands without the "packet" prefix on each.
`,
	CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	SilenceErrors:     true,
	SilenceUsage:      true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if term != nil {
			saveTerm = term
		}
		term = terminal.New(cmd)
	},
}

func Execute(args []string) (err error) {
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	term.Close()
	return err
}

// expandMessageID searches all messages in the current directory for those
// whose local message ID matches the supplied input.  If it finds exactly one,
// it returns the full message ID.  If it finds more than one, it returns an
// error.  If it doesn't find any, and remoteOK is true, it searches remote
// message IDs as well.  Again, if it finds exactly one match, it returns the
// full local message ID of that message.  If it finds more than one, or none,
// it returns an error.
func expandMessageID(in string, remoteOK bool) (lmi string, err error) {
	var (
		seq     int
		matches []string
	)
	inUC := strings.ToUpper(in)
	if incident.MessageExists(inUC) {
		return inUC, nil
	}
	if remoteOK {
		if lmi := incident.LMIForRMI(inUC); lmi != "" {
			return lmi, nil
		}
	}
	if seq, err = strconv.Atoi(in); err != nil || seq <= 0 {
		return "", fmt.Errorf("%q is not a valid message ID or number", in)
	}
	if matches, err = incident.SeqToLMI(seq, false); err != nil {
		return "", err
	}
	if len(matches) == 0 && remoteOK {
		matches, _ = incident.SeqToLMI(seq, true)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no such message %q", in)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%q is ambiguous: it could be %s", in, strings.Join(matches, ", "))
	}
}

// expandFieldName finds the message field that (best) matches the supplied
// field name.  If loose is true, it can be a partial, heuristic match.
func expandFieldName(fields []*message.Field, in string, loose bool) (*message.Field, error) {
	// First priority is a match on PIFO tag.
	for _, f := range fields {
		if f.PIFOTag == in {
			return f, nil
		}
	}
	// Remaining comparisons are case-sensitive if the input contains any
	// uppercase letters.
	var caseSensitive bool
	var compare func(string, string) bool
	if strings.IndexFunc(in, func(r rune) bool { return r >= 'A' && r <= 'Z' }) >= 0 {
		compare, caseSensitive = func(a, b string) bool { return a == b }, true
	} else {
		compare, caseSensitive = strings.EqualFold, false
	}
	// Second priority is a case-insensitive match on full field name.
	for _, f := range fields {
		if compare(f.Label, in) {
			return f, nil
		}
	}
	// To avoid the need for quoting, we will also accept a case-insensitive
	// match on the full field name with spaces removed.
	if !strings.Contains(in, " ") {
		for _, f := range fields {
			if compare(strings.ReplaceAll(f.Label, " ", ""), in) {
				return f, nil
			}
		}
	}
	// Unless the loose flag is set, those are the only options.
	if !loose {
		return nil, fmt.Errorf("no such field %q (loose matches are not allowed in batch mode)", in)
	}
	// Now we look for fields whose name contains the same characters as the
	// input, in the same order, but also contains additional characters.
	// We return the field that has the smallest number of unmatched
	// capital letters in its name, and among those, the one with the
	// smallest number of unmatched characters.
	var match *message.Field
	var missedUC, missedCH int
	for _, f := range fields {
		if mUC, mCH, ok := matchFieldName(f.Label, in, caseSensitive); ok {
			if match == nil || mUC < missedUC || (mUC == missedUC && mCH < missedCH) {
				match, missedUC, missedCH = f, mUC, mCH
			}
		}
	}
	if match == nil {
		return nil, fmt.Errorf("no such field %q", in)
	}
	return match, nil
}

// matchFieldName is a recursive function that determines whether the input is a
// valid shortening of the field name, and returns the heuristic scoring if so.
// It's a pretty expensive algorithm, but at human time scales it's negligible.
func matchFieldName(fname, in string, caseSensitive bool) (mUC, mCH int, ok bool) {
	if in == "" && fname == "" {
		// Nothing left of either string.  Perfect match.
		return 0, 0, true
	}
	if in == "" {
		// Nothing left of in, but we still have some fname.  Compute
		// the score.  Use a recursive call to get the score after
		// removing the first character of fname, then add the score
		// for that character.
		mUC, mCH, _ = matchFieldName(fname[1:], in, caseSensitive)
		if fname[0] >= 'A' && fname[0] <= 'Z' {
			mUC++
		}
		mCH++
		return mUC, mCH, true
	}
	if fname == "" {
		// Nothing left of fname, but we still have some in.  Not a
		// match at all.
		return 0, 0, false
	}
	var mUC1, mCH1, mUC2, mCH2 int
	var ok1, ok2 bool
	// If the lead characters of fname and in match, calculate the score
	// based on matching those two.
	if fname[0] == in[0] || (!caseSensitive && downcase(fname[0]) == downcase(in[0])) {
		mUC1, mCH1, ok1 = matchFieldName(fname[1:], in[1:], caseSensitive)
	}
	// Whether the lead characters match or not, also calculate the score
	// assuming they don't.
	mUC2, mCH2, ok2 = matchFieldName(fname[1:], in, caseSensitive)
	if fname[0] >= 'A' && fname[0] <= 'Z' {
		mUC2++
	}
	mCH2++
	// Return the better of the two scores.
	if !ok1 && !ok2 {
		return 0, 0, false
	}
	if !ok1 {
		return mUC2, mCH2, ok2
	}
	if !ok2 || mUC1 < mUC2 || (mUC1 == mUC2 && mCH1 < mCH2) {
		return mUC1, mCH1, ok1
	}
	return mUC2, mCH2, ok2
}
func downcase(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b + 'A' - 'a'
	}
	return b
}

func tokenizeLine(line string) (args []string) {
	var partial string

	for line != "" {
		idx := strings.IndexAny(line, " \t\f\r\n'\"<>")
		if idx < 0 {
			partial += line
			args = append(args, partial)
			return args
		}
		if line[idx] == '\'' || line[idx] == '"' {
			partial += line[:idx]
			idx2 := strings.IndexByte(line[idx+1:], line[idx])
			if idx2 < 0 {
				partial += line[idx+1:]
				args = append(args, partial)
				return args
			}
			idx2 += idx + 1 // make it an offset into line
			partial += line[idx+1 : idx2]
			line = line[idx2+1:]
			continue
		}
		if line[idx] == '<' || line[idx] == '>' {
			if partial != "" {
				args, partial = append(args, partial), ""
			}
			args, line = append(args, line[idx:idx+1]), line[idx+1:]
			continue
		}
		partial += line[:idx]
		if partial != "" {
			args, partial = append(args, partial), ""
		}
		line = line[idx+1:]
	}
	if partial != "" {
		args = append(args, partial)
	}
	return args
}
