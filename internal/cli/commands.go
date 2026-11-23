package cli

import (
	"fmt"
	"os"

	ts "timesheet-cli/internal/timesheet"
)

func list(a *app, month string) error {
	entries, err := a.svc.List(month)
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, a.p.c.cyan("Timesheet: "+a.name+" -> "+a.repo.Path+"  ("+month+")"))
	ts.WriteTable(a.out, entries)
	return nil
}

// rm empties check in, check out and task of one day (the work place stays). Asks first.
func rm(a *app, f flags, pos []string) error {
	date := *f.date
	if len(pos) > 1 {
		return usageError("rm takes at most one [date]")
	}
	if len(pos) == 1 {
		date = pos[0]
	}
	e, err := a.svc.Get(date)
	if err != nil {
		return err
	}
	if !e.Filled() {
		fmt.Fprintf(a.out, "nothing to clear on %s\n", date)
		return nil
	}
	if !*f.yes {
		ok, err := a.p.confirm("Clear " + date + " (" + summary(e) + ")?")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(a.out, "kept")
			return nil
		}
	}
	if err := a.withLock(func() error { return a.svc.Clear(date) }); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "cleared %s in %s\n", date, a.name)
	return nil
}

func export(a *app, month, to string) error {
	entries, err := a.svc.List(month)
	if err != nil {
		return err
	}
	if to == "" {
		return ts.WriteCSV(a.out, entries)
	}
	f, err := os.Create(to)
	if err != nil {
		return err
	}
	if err := ts.WriteCSV(f, entries); err != nil {
		f.Close()
		return err
	}
	fmt.Fprintf(a.out, "wrote %d rows to %s\n", len(entries), to)
	return f.Close()
}
