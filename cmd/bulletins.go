package cmd

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rothskeller/packet-cmd/config"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(bulletinsCmd)
	bulletinsCmd.Flags().DurationP("frequency", "f", time.Hour, "time between checks")
	bulletinsCmd.Flag("frequency").DefValue = "1h"
	bulletinsCmd.Flags().BoolP("now", "n", false, "force check at next connect")
	bulletinsCmd.Flags().BoolP("stop", "s", false, "stop periodic checks")
	bulletinsCmd.MarkFlagsMutuallyExclusive("now", "frequency", "stop")
}

var areaRE = regexp.MustCompile(`(?i)^(?:[A-Z][A-Z0-9]{0,7}@)?[A-Z][A-Z]{0,7}$`)

var bulletinsCmd = &cobra.Command{
	Use:                   "bulletins [--frequency f | --now | --stop] [«area»...]",
	DisableFlagsInUseLine: true,
	Aliases:               []string{"bulletin", "bull", "b"},
	Short:                 "Schedule periodic checks for bulletins",
	Long: `The "bulletins" command controls the scheduling of periodic checks for
bulletins.  During each BBS connection that retrieves non-immediate private
messages, the connection will also check for bulletins in each area where a
scheduled check is due.  (Bulletins are never checked during send-only or
immediate-only connections.)

If any «area»s are listed on the command line, the schedules for bulletin
checks in those areas will be updated.  The «area»s are names of bulletin areas
(e.g., "XSCEVENT"), optionally preceded by a recipient name and an at-sign
(e.g., "XND@XSC").  The schedules for them will be updated as follows:
  - If the --now (-n) flag is given, the areas will be checked during the next
    connection, and their existing schedules will resume unchanged after that.
  - If the --stop (-s) flag is given, no further checks will be performed for
    the areas.
  - If the --frequency (-f) flag is given, the areas will be checked at that
    frequency (e.g., "30m" or "2h15m").
  - If none of these three flags is given, the areas will be checked hourly.

If no «area»s are listed on the command line, but one of the --now, --stop, or
--frequency flags is given, the corresponding change will be made to all areas
that currently have scheduled checks.

In all cases, the command prints the resulting schedule of bulletin checks.
If there are no «area»s on the command line and none of the three flags,
printing the schedule is the only thing the command does.  When running in
interactive (--human) mode, the table is formatted for human viewing.  When
running in noninteractive (--batch) mode, the table is in CSV format.
`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			freq    time.Duration
			setFreq bool
			stop    bool
			now     bool
			changed bool
		)
		// Parse the arguments.
		if freq, err = cmd.Flags().GetDuration("frequency"); err != nil || freq <= 0 {
			return fmt.Errorf("--frequency value is not a valid duration")
		}
		if cmd.Flag("frequency").Changed {
			setFreq = true
		}
		stop, _ = cmd.Flags().GetBool("stop")
		now, _ = cmd.Flags().GetBool("now")
		if len(args) != 0 && !stop && !now {
			setFreq = true
		}
		// If we didn't get any named areas but we're supposed to do
		// something, do them all.
		if len(args) == 0 && (setFreq || stop || now) {
			args = make([]string, 0, len(config.C.Bulletins))
			for area := range config.C.Bulletins {
				args = append(args, area)
			}
		} else {
			// We were given some area names.  Make sure they're
			// valid ones.  Also normalize them.
			for i, arg := range args {
				if !areaRE.MatchString(arg) {
					return fmt.Errorf("%q is not a valid area name", arg)
				}
				args[i] = strings.Replace(strings.ToUpper(arg), "@ALL", "@", 1)
			}
		}
		// Make the requested changes.
		if config.C.Bulletins == nil {
			config.C.Bulletins = make(map[string]*config.BulletinConfig)
		}
		for _, area := range args {
			if bc, ok := config.C.Bulletins[area]; ok {
				if now && !bc.LastCheck.IsZero() {
					bc.LastCheck = time.Time{}
					changed = true
				}
				if setFreq && bc.Frequency != freq {
					bc.Frequency = freq
					changed = true
				}
				if stop {
					delete(config.C.Bulletins, area)
					changed = true
				}
			} else {
				if now {
					config.C.Bulletins[area] = &config.BulletinConfig{}
					changed = true
				} else if setFreq {
					config.C.Bulletins[area] = &config.BulletinConfig{Frequency: freq}
					changed = true
				}
			}
		}
		if changed {
			config.SaveConfig()
		}
		// Report the bulletin schedule.
		term.BulletinScheduleTable()
		return nil
	},
}
