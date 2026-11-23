package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	ts "timesheet-cli/internal/timesheet"
	"timesheet-cli/internal/xlsx"
)

// add fills the row of one day (today by default). With --in/--out/--place/--task or one positional
// [task] it applies just those fields; with none of them it asks for every field, pre-filled
// with what the row already holds.
func add(a *app, f flags, pos []string) error {
	if len(pos) > 1 || (len(pos) == 1 && f.set["task"]) {
		return usageError("add takes at most one [task]")
	}
	date := *f.date
	e, err := a.svc.Get(date)
	if err != nil {
		return err
	}
	var patch ts.Patch
	if f.set["in"] || f.set["out"] || f.set["place"] || f.set["task"] || len(pos) == 1 {
		patch = flagPatch(f, pos)
	} else if patch, err = addInteractive(a, date, e); err != nil {
		return err
	}

	var after ts.Entry
	err = a.withLock(func() (err error) { _, after, err = a.svc.Log(date, patch); return })
	if errors.Is(err, ts.ErrNoChange) {
		fmt.Fprintf(a.out, "no changes for %s in %s\n", date, a.name)
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%s %s\n", a.p.c.green("saved "+date+" in "+a.name+":"), summary(after))
	remind(a, date)
	return nil
}

func flagPatch(f flags, pos []string) ts.Patch {
	var p ts.Patch
	if f.set["in"] {
		p.CheckIn = f.in
	}
	if f.set["out"] {
		p.CheckOut = f.out
	}
	if f.set["place"] {
		p.WorkPlace = f.place
	}
	if f.set["task"] {
		p.Task = f.task
	}
	if len(pos) == 1 {
		p.Task = &pos[0]
	}
	return p
}

func summary(e ts.Entry) string {
	times := "-"
	if e.CheckIn != "" || e.CheckOut != "" {
		times = e.CheckIn + " - " + e.CheckOut
	}
	task := e.Task
	if task == "" {
		task = "-"
	}
	return strings.Join([]string{times, e.WorkPlace, task}, " | ")
}

func addInteractive(a *app, date string, e ts.Entry) (ts.Patch, error) {
	c := a.p.c
	day := date
	if t, err := time.Parse("2006-01-02", date); err == nil {
		day = t.Format("Mon 2006-01-02")
	}
	fmt.Fprintln(a.out, c.cyan("Timesheet: "+a.name+" -> "+a.repo.Path))
	fmt.Fprintln(a.out, c.cyan(day))
	if e.Filled() {
		fmt.Fprintln(a.out, c.dim("Already logged: "+summary(e)+"  (Enter keeps a value, - clears it)"))
	}
	now := time.Now()
	in, err := a.p.askClock("Check in (HH:MM or now)", e.CheckIn, now)
	if err != nil {
		return ts.Patch{}, err
	}
	out, err := a.p.askClock("Check out (HH:MM or now)", e.CheckOut, now)
	if err != nil {
		return ts.Patch{}, err
	}
	place, err := a.p.ask("Work place", e.WorkPlace)
	if err != nil {
		return ts.Patch{}, err
	}
	task, err := a.p.ask("Task", e.Task)
	if err != nil {
		return ts.Patch{}, err
	}
	return ts.Patch{CheckIn: &in, CheckOut: &out, WorkPlace: &place, Task: &task}, nil
}

// withLock runs a write and, when Excel's lock file is present, asks before writing anyway.
func (a *app) withLock(write func() error) error {
	err := write()
	var le *xlsx.LockedError
	if !errors.As(err, &le) {
		return err
	}
	fmt.Fprintln(a.out, a.p.c.red(le.Error()+". Saving now can overwrite changes made in Excel."))
	ok, cerr := a.p.confirm("Write anyway?")
	if cerr != nil {
		return cerr
	}
	if !ok {
		return fmt.Errorf("not written: %w", le)
	}
	a.repo.IgnoreLock = true
	return write()
}

// remind lists earlier days of the month that are still empty: the ones easy to forget to flag.
func remind(a *app, date string) {
	rows, err := a.svc.List(date[:7])
	if err != nil {
		return
	}
	today := time.Now().Format("2006-01-02")
	var empty []string
	for _, r := range rows {
		if r.Date < today && r.Date != date && !r.Filled() {
			empty = append(empty, r.Date)
		}
	}
	if len(empty) == 0 {
		return
	}
	more := ""
	if len(empty) > 6 {
		more = fmt.Sprintf(" (+%d more)", len(empty)-6)
		empty = empty[:6]
	}
	fmt.Fprintln(a.out, a.p.c.dim("Still empty: "+strings.Join(empty, ", ")+more))
}
