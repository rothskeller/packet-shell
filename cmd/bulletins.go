package cmd

import (
	"regexp"
	"strings"
	"time"

	"github.com/rothskeller/packet-shell/cio"
	"github.com/rothskeller/packet-shell/config"

	"github.com/spf13/pflag"
)

const bulletinsSlug = `Schedule periodic checks for bulletins`
const bulletinsHelp = `
usage: packet bulletins ⇥[flags] [«area»...]
  -f, --frequency «interval»  ⇥set interval between checks (default 1h)
  -n, --now                   ⇥force check at next connect
  -s, --stop                  ⇥stop periodic checks
These flags are mutually exclusive.

The "bulletins" command (or "b", "bull", or "bulletin") controls the scheduling of periodic checks for bulletins.  During each BBS connection that retrieves non-immediate private messages, the connection will also check for bulletins in each area where a scheduled check is due.  (Bulletins are never checked during send-only or immediate-only connections.)

If any «area»s are listed on the command line, the schedules for bulletin checks in those areas will be updated.  The «area»s are names of shared mailboxes (e.g., "XSCEVENT") or «category»@«distribution» bulletin addressed (e.g., "XND@XSC").  The schedules for them will be updated as follows:
  - ⇥If the --now (-n) flag is given, the areas will be checked during the next connection, and their existing schedules will resume unchanged after that.
  - ⇥If the --stop (-s) flag is given, no further checks will be performed for the areas.
  - ⇥If the --frequency (-f) flag is given, the areas will be checked at that frequency (e.g., "30m" or "2h15m").
  - ⇥If none of these three flags is given, the areas will be checked hourly.

If no «area»s are listed on the command line, but one of the --now, --stop, or --frequency flags is given, the corresponding change will be made to all areas that currently have scheduled checks.

In all cases, the command prints the resulting schedule of bulletin checks. If there are no «area»s on the command line and none of the three flags, printing the schedule is the only thing the command does.  The table will be in human format if stdout is a terminal, and in CSV format otherwise.
`

var areaRE = regexp.MustCompile(`(?i)^(?:[A-Z][A-Z0-9]{0,7}@)?[A-Z][A-Z]{0,7}$`)

func cmdBulletins(args []string) (err error) {
	var (
		frequency time.Duration
		now       bool
		stop      bool
		flags     = pflag.NewFlagSet("bulletins", pflag.ContinueOnError)
	)
	flags.DurationVarP(&frequency, "frequency", "f", time.Hour, "time between checks")
	flags.BoolVarP(&now, "now", "n", false, "force check at next connect")
	flags.BoolVarP(&stop, "stop", "s", false, "stop periodic checks")
	flags.Usage = func() {} // we do our own
	if err = flags.Parse(args); err == pflag.ErrHelp {
		return cmdHelp([]string{"bulletins"})
	} else if err != nil {
		cio.Error(err.Error())
		return usage(bulletinsHelp)
	} else if err = gaveMutuallyExclusiveFlags(flags, "frequency", "now", "stop"); err != nil {
		cio.Error(err.Error())
		return usage(bulletinsHelp)
	}
	if frequency <= 0 {
		cio.Error("--frequency must be a positive duration")
		return usage(bulletinsHelp)
	}
	args = flags.Args()
	for i, arg := range args {
		if !areaRE.MatchString(arg) {
			cio.Error("%q is not a valid shared mailbox or bulletin address", arg)
			return usage(bulletinsHelp)
		}
		args[i] = strings.Replace(strings.ToUpper(arg), "@ALL", "@", 1)
	}
	if !flags.Lookup("frequency").Changed && !stop && !now && len(args) == 0 {
		frequency = 0 // don't change
	}
	return doBulletins(frequency, stop, now, args)
}

func doBulletins(frequency time.Duration, stop, now bool, areas []string) (err error) {
	var changed bool

	// If we didn't get any named areas but we're supposed to do
	// something, do them all.
	if len(areas) == 0 && (frequency != 0 || stop || now) {
		areas = make([]string, 0, len(config.C.Bulletins))
		for area := range config.C.Bulletins {
			areas = append(areas, area)
		}
	}
	// Make the requested changes.
	if config.C.Bulletins == nil {
		config.C.Bulletins = make(map[string]*config.BulletinConfig)
	}
	for _, area := range areas {
		if bc, ok := config.C.Bulletins[area]; ok {
			if now && !bc.LastCheck.IsZero() {
				bc.LastCheck = time.Time{}
				changed = true
			}
			if frequency != 0 && bc.Frequency != frequency {
				bc.Frequency = frequency
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
			} else if frequency != 0 && !stop {
				config.C.Bulletins[area] = &config.BulletinConfig{Frequency: frequency}
				changed = true
			}
		}
	}
	if changed {
		config.SaveConfig()
	}
	// Report the bulletin schedule.
	cio.BulletinScheduleTable()
	return nil
}
