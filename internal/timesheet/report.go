package timesheet

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"
)

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func weekday(date string) string {
	if t, err := time.Parse(dateLayout, date); err == nil {
		return t.Format("Mon")
	}
	return "   "
}

// WriteTable prints one line per day and a footer counting the days that have something logged.
func WriteTable(w io.Writer, entries []Entry) {
	filled := 0
	for _, e := range entries {
		if e.Filled() {
			filled++
		}
		fmt.Fprintf(w, "%s %s  %5s  %5s  %-20s  %s\n", e.Date, weekday(e.Date), dash(e.CheckIn), dash(e.CheckOut), e.WorkPlace, dash(e.Task))
	}
	fmt.Fprintf(w, "Filled: %d of %d days\n", filled, len(entries))
}

func WriteCSV(w io.Writer, entries []Entry) error {
	cw := csv.NewWriter(w)
	cw.Write([]string{"date", "check_in", "check_out", "work_place", "task"})
	for _, e := range entries {
		cw.Write([]string{e.Date, e.CheckIn, e.CheckOut, e.WorkPlace, e.Task})
	}
	cw.Flush()
	return cw.Error()
}
