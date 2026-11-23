package timesheet

// Entry is one day's row in a timesheet file. Date is YYYY-MM-DD; CheckIn and CheckOut are "HH:MM" or empty.
type Entry struct {
	Date      string
	CheckIn   string
	CheckOut  string
	WorkPlace string
	Task      string
}

// Filled reports whether the day has anything logged. WorkPlace alone does not count:
// the template pre-fills it on every row.
func (e Entry) Filled() bool { return e.CheckIn != "" || e.CheckOut != "" || e.Task != "" }
