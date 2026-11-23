package timesheet

import (
	"errors"
	"testing"
	"time"
)

type fakeRepo struct{ rows map[string]Entry }

func (f *fakeRepo) Get(d string) (Entry, error) {
	e, ok := f.rows[d]
	if !ok {
		return Entry{}, ErrNoRow
	}
	return e, nil
}
func (f *fakeRepo) Save(e Entry) error { f.rows[e.Date] = e; return nil }
func (f *fakeRepo) Clear(d string) error {
	e := f.rows[d]
	f.rows[d] = Entry{Date: d, WorkPlace: e.WorkPlace}
	return nil
}
func (f *fakeRepo) List(m string) ([]Entry, error) { return nil, nil }

func newSvc() (*Service, *fakeRepo) {
	r := &fakeRepo{rows: map[string]Entry{"2026-11-20": {Date: "2026-11-20", WorkPlace: "Office"}}}
	s := NewService(r)
	s.Now = func() time.Time { return time.Date(2026, 11, 20, 9, 5, 0, 0, time.UTC) }
	return s, r
}

func p(s string) *string { return &s }

func TestParseClock(t *testing.T) {
	now := time.Date(2026, 1, 1, 9, 5, 0, 0, time.UTC)
	ok := map[string]string{"7:30": "07:30", "07:30": "07:30", "7.30": "07:30", " 16:05 ": "16:05", "now": "09:05", "NOW": "09:05", "": "", "-": ""}
	for in, want := range ok {
		if got, err := ParseClock(in, now); err != nil || got != want {
			t.Errorf("%q -> %q, %v (want %q)", in, got, err, want)
		}
	}
	for _, bad := range []string{"24:00", "7:60", "730", "abc", "7:3"} {
		if _, err := ParseClock(bad, now); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
}

func TestLogPatchSemantics(t *testing.T) {
	s, r := newSvc()
	before, after, err := s.Log("2026-11-20", Patch{CheckIn: p("7.31")})
	if err != nil || before.CheckIn != "" || after.CheckIn != "07:31" || after.WorkPlace != "Office" {
		t.Fatalf("check in only: %+v %+v %v", before, after, err)
	}
	// nil fields keep what is stored
	if _, after, _ = s.Log("2026-11-20", Patch{Task: p("  fix\n login  ")}); after.CheckIn != "07:31" || after.Task != "fix login" {
		t.Fatalf("keep + task cleanup: %+v", after)
	}
	if _, after, _ = s.Log("2026-11-20", Patch{CheckOut: p("now")}); after.CheckOut != "09:05" {
		t.Fatalf("now: %+v", after)
	}
	// "-" clears
	if _, after, _ = s.Log("2026-11-20", Patch{Task: p("-")}); after.Task != "" {
		t.Fatalf("clear task: %+v", after)
	}
	if _, _, err = s.Log("2026-11-20", Patch{CheckIn: p("07:31")}); !errors.Is(err, ErrNoChange) {
		t.Fatalf("same value must be ErrNoChange, got %v", err)
	}
	if r.rows["2026-11-20"].CheckOut != "09:05" {
		t.Fatal("not saved")
	}
}

func TestLogValidation(t *testing.T) {
	s, r := newSvc()
	if _, _, err := s.Log("2026-11-20", Patch{CheckIn: p("16:00"), CheckOut: p("08:00")}); err == nil {
		t.Fatal("check out before check in must fail")
	}
	if r.rows["2026-11-20"].CheckIn != "" {
		t.Fatal("a rejected patch must not be saved")
	}
	if _, _, err := s.Log("2026-11-20", Patch{CheckIn: p("25:00")}); err == nil {
		t.Fatal("bad time must fail")
	}
	if _, _, err := s.Log("20-11-2026", Patch{CheckIn: p("08:00")}); err == nil {
		t.Fatal("bad date must fail")
	}
	if _, _, err := s.Log("2026-11-21", Patch{CheckIn: p("08:00")}); !errors.Is(err, ErrNoRow) {
		t.Fatalf("unknown row: %v", err)
	}
	if _, err := s.List("2026/11"); err == nil {
		t.Fatal("bad month must fail")
	}
}
