package cio

import (
	"encoding/csv"
	"os"
	"time"
)

type ListItem struct {
	Handling string // "I", "P", "R", "B" for bulletin
	Flag     string // "DRAFT", "QUEUE", "NO RCPT", "HAVE RCPT", "NEW"
	Time     time.Time
	From     string
	LMI      string
	To       string
	Subject  string
	NoHeader bool
}

var (
	listCW       *csv.Writer
	listItemSeen bool
)

func ListMessage(li *ListItem) {
	if !OutputIsTerm {
		listMessageCSV(li)
	} else {
		listMessageTable(li)
	}
}

func listMessageCSV(li *ListItem) {
	if listCW == nil {
		listCW = csv.NewWriter(os.Stdout)
		if !li.NoHeader {
			listCW.Write([]string{"FLAG", "TIME", "FROM", "LMI", "TO", "SUBJECT"})
		}
	}
	var tstr string
	if !li.Time.IsZero() {
		tstr = li.Time.Format(time.RFC3339)
	}
	listCW.Write([]string{li.Flag, tstr, li.From, li.LMI, li.To, li.Subject})
}

func listMessageTable(li *ListItem) {
	var lineColor int

	clearStatus()
	if !listItemSeen && !li.NoHeader {
		print(colorWhite, "TIME  FROM        LOCAL ID    TO         SUBJECT")
		print(0, "\n")
	}
	listItemSeen = true
	switch li.Handling {
	case "B":
		lineColor = colorBulletin
	case "I":
		lineColor = colorImmediate
	case "P":
		lineColor = colorPriority
	}
	if !li.Time.IsZero() {
		now := time.Now()
		if now.Year() != li.Time.Year() {
			print(lineColor, li.Time.Format("2006  "))
		} else if now.Month() != li.Time.Month() || now.Day() != li.Time.Day() {
			print(lineColor, li.Time.Format("01/02 "))
		} else {
			print(lineColor, li.Time.Format("15:04 "))
		}
	} else if li.Flag == "QUEUE" {
		print(colorWarningBG, li.Flag)
		print(lineColor, " ")
	} else if li.Flag == "DRAFT" {
		print(colorAlertBG, li.Flag)
		print(lineColor, " ")
	} else {
		print(lineColor, "      ")
	}
	if li.Flag == "NO RCPT" {
		print(colorWarningBG, li.Flag)
		print(lineColor, "     ")
	} else if li.From != "" {
		print(lineColor, setLength(li.From, 9)+" → ")
	} else {
		print(lineColor, "            ")
	}
	print(lineColor, setLength(li.LMI, 9))
	if li.Flag == "HAVE RCPT" {
		print(lineColor, " → ")
		print(colorSuccessBG, setMaxLength(li.To, 9))
		if len(li.To) < 9 {
			print(lineColor, spaces[:11-len(li.To)])
		} else {
			print(lineColor, "  ")
		}
	} else if li.To != "" {
		print(lineColor, " → "+setLength(li.To, 9)+"  ")
	} else if li.Flag == "NEW" {
		print(lineColor, "   ")
		print(colorWarningBG, "NEW")
		print(lineColor, "        ")
	} else {
		print(lineColor, "              ")
	}
	print(lineColor, setMaxLength(li.Subject, Width-41))
	print(0, "\n")
}

func EndMessageList(s string) {
	if listCW != nil {
		listCW.Flush()
		listCW = nil
	} else {
		clearStatus()
		if !listItemSeen && s != "" {
			print(0, s)
			print(0, "\n")
		}
		listItemSeen = false
	}
}
