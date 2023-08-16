package cio

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/rothskeller/packet-shell/config"

	"golang.org/x/exp/maps"
)

func BulletinScheduleTable(bulletins map[string]*config.BulletinConfig) {
	if OutputIsTerm {
		bulletinsTable(bulletins)
	} else {
		bulletinsCSV(bulletins)
	}
}

func bulletinsCSV(bulletins map[string]*config.BulletinConfig) {
	var (
		cw    *csv.Writer
		areas = maps.Keys(config.C.Bulletins)
	)
	if len(areas) == 0 {
		return
	}
	sort.Strings(areas)
	cw = csv.NewWriter(os.Stdout)
	cw.Write([]string{"AREA", "SCHEDULE", "LAST CHECK"})
	for _, area := range areas {
		bc := config.C.Bulletins[area]
		fd := bc.Frequency.String()
		var lc string
		if !bc.LastCheck.IsZero() {
			lc = bc.LastCheck.Format(time.RFC3339)
		}
		cw.Write([]string{area, fd, lc})
	}
	cw.Flush()
}

func bulletinsTable(bulletins map[string]*config.BulletinConfig) {
	var (
		col1  = []string{"AREA"}
		col2  = []string{"SCHEDULE"}
		col3  = []string{"LAST CHECK"}
		len1  = 4
		len2  = 8
		areas = maps.Keys(config.C.Bulletins)
	)
	clearStatus()
	if len(areas) == 0 {
		io.WriteString(os.Stdout, "No bulletin checks are scheduled in any area.\n")
		return
	}
	sort.Strings(areas)
	col1 = append(col1, areas...)
	for i, area := range col1 {
		if i == 0 {
			continue
		}
		len1 = max(len1, len(area))
		bc := config.C.Bulletins[area]
		if bc.Frequency == 0 {
			col2 = append(col2, "one time")
		} else {
			col2 = append(col2, "every "+fmtDuration(bc.Frequency))
		}
		len2 = max(len2, len(col2[i]))
		if bc.LastCheck.IsZero() {
			col3 = append(col3, "never")
		} else {
			col3 = append(col3, fmtDuration(time.Since(bc.LastCheck))+" ago")
		}
	}
	for i, area := range col1 {
		var color int
		if i == 0 {
			color = colorWhite
		}
		print(color, setLength(area, len1+2))
		print(color, setLength(col2[i], len2+2))
		print(color, col3[i])
		print(0, "\n")
	}
}

func fmtDuration(d time.Duration) (s string) {
	if d >= time.Hour {
		var h time.Duration
		h, d = d/time.Hour, d%time.Hour
		s = fmt.Sprintf("%dh", h)
	}
	if d >= time.Minute {
		s += fmt.Sprintf("%dm", d/time.Minute)
	}
	if s == "" {
		s = "1m"
	}
	return s
}
