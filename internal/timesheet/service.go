package timesheet

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const dateLayout = "2006-01-02"

var (
	// ErrNoRow means the timesheet file has no row for that date (weekend, or outside the template).
	ErrNoRow = errors.New("no row for that date in the timesheet")
	// ErrNoChange means the requested values equal what is already stored.
	ErrNoChange = errors.New("nothing changed")

	monthRe = regexp.MustCompile(`^\d{4}-\d{2}$`)
)

// Repository is the storage contract the service depends on.
type Repository interface {
	Get(date string) (Entry, error)     // ErrNoRow (wrapped) when the file has no row for date
	Save(e Entry) error                 // writes CheckIn, CheckOut, Task; WorkPlace only when non-empty
	Clear(date string) error            // empties CheckIn, CheckOut, Task; keeps WorkPlace
	List(month string) ([]Entry, error) // every row dated in month (YYYY-MM), by date, filled or not
}

// Patch holds the fields to change. A nil field keeps the stored value; "-" clears it.
type Patch struct{ CheckIn, CheckOut, WorkPlace, Task *string }

type Service struct {
	repo Repository
	Now  func() time.Time
}

func NewService(repo Repository) *Service { return &Service{repo: repo, Now: time.Now} }

func checkDate(date string) error {
	if _, err := time.Parse(dateLayout, date); err != nil {
		return fmt.Errorf("date must be YYYY-MM-DD")
	}
	return nil
}

// cleanText makes a task safe for one cell: one line, no control characters.
func cleanText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func (s *Service) Get(date string) (Entry, error) {
	if err := checkDate(date); err != nil {
		return Entry{}, err
	}
	return s.repo.Get(date)
}

// Log applies p on top of the stored row for date and saves it. It returns the row before and after.
func (s *Service) Log(date string, p Patch) (before, after Entry, err error) {
	if before, err = s.Get(date); err != nil {
		return
	}
	after = before
	now := s.Now()
	if p.CheckIn != nil {
		if after.CheckIn, err = ParseClock(*p.CheckIn, now); err != nil {
			return
		}
	}
	if p.CheckOut != nil {
		if after.CheckOut, err = ParseClock(*p.CheckOut, now); err != nil {
			return
		}
	}
	if p.WorkPlace != nil && cleanText(*p.WorkPlace) != "" {
		after.WorkPlace = cleanText(*p.WorkPlace)
	}
	if p.Task != nil {
		after.Task = cleanText(*p.Task)
		if after.Task == "-" {
			after.Task = ""
		}
	}
	if after.CheckIn != "" && after.CheckOut != "" && after.CheckOut < after.CheckIn {
		err = fmt.Errorf("check out %s is before check in %s", after.CheckOut, after.CheckIn)
		return
	}
	if after == before {
		err = ErrNoChange
		return
	}
	err = s.repo.Save(after)
	return
}

func (s *Service) Clear(date string) error {
	if err := checkDate(date); err != nil {
		return err
	}
	return s.repo.Clear(date)
}

func (s *Service) List(month string) ([]Entry, error) {
	if !monthRe.MatchString(month) {
		return nil, fmt.Errorf("month must be YYYY-MM")
	}
	return s.repo.List(month)
}
